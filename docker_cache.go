package main

import (
	"flag"
	"fmt"
	"github.com/samalba/dockerclient"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
)

type Config struct {
	Id             string
	CacheURL       string
	DockerURL      string
	UpdateInterval time.Duration
}

type RuntimeInfo struct {
	Id     string
	Cache  *Cache
	Docker *dockerclient.DockerClient
	Ttl    time.Duration
}

// Callback used to listen to Docker's events
func dockerEventCallback(event *dockerclient.Event, args ...interface{}) {
	rtInfo := args[0].(*RuntimeInfo)
	eventName := strings.ToLower(event.Status)
	eventErr := func(err error) {
		log.Println("Cannot process event:", eventName, event.Id, err)
	}
	if eventName == "start" || eventName == "restart" {
		containerInfo, err := rtInfo.Docker.InspectContainer(event.Id)
		if err != nil {
			eventErr(err)
			return
		}
		err = rtInfo.Cache.AddContainer(containerInfo)
		if err != nil {
			eventErr(err)
		}
		log.Println("Reported 1 new container to the Cache:", event.Id)
		return
	}
	if eventName == "die" {
		containerInfo, err := rtInfo.Docker.InspectContainer(event.Id)
		if err != nil {
			eventErr(err)
			return
		}
		err = rtInfo.Cache.DeleteContainer(containerInfo)
		if err != nil {
			eventErr(err)
		}
		log.Println("Removed 1 container from the Cache:", event.Id)
		return
	}
}

func update(rtInfo *RuntimeInfo) {
	containers, err := rtInfo.Docker.ListContainers(false)
	if err != nil {
		log.Println("Cannot list Docker containers:", err,
			"(will retry later)")
		return
	}
	for _, container := range containers {
		containerInfo, err := rtInfo.Docker.InspectContainer(container.Id)
		if err != nil {
			// If we cannot get the info of the container, it may have been
			// removed already. Ignoring to avoid race condition.
			continue
		}
		err = rtInfo.Cache.SetContainerInfo(containerInfo)
		if err != nil {
			log.Println("Cannot write in the Cache (SetContainerInfo):", err)
			return
		}
	}
	err = rtInfo.Cache.SetContainersList(containers)
	if err != nil {
		log.Println("Cannot write in the Cache (SetContainersList):", err)
		return
	}
	log.Println("Reported", len(containers), "containers to the Cache")
	version, err := rtInfo.Docker.Version()
	if err != nil {
		log.Println("Cannot get Docker's version:", err)
		return
	}
	versionString := fmt.Sprintf("%s;git-%s;%s", version.Version,
		version.GitCommit, version.GoVersion)
	rtInfo.Cache.SetHostParam("docker_version", versionString)
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
	return hostname
}

func parseFlags() *Config {
	config := &Config{}
	flag.StringVar(&config.Id, "id", getHostname(),
		"Id that will identify this host on the Cache (it should be unique to avoid conflicts)")
	flag.StringVar(&config.CacheURL, "cache", "redis://localhost:6379",
		"Cache URL which will receive the data")
	flag.StringVar(&config.DockerURL, "docker", "unix:///var/run/docker.sock",
		"Docker daemon URL to read the data")
	flag.DurationVar(&config.UpdateInterval, "updateInterval",
		time.Duration(120)*time.Second,
		"Interval for polling data from Docker")
	flag.Parse()
	return config
}

func main() {
	random := rand.New(rand.NewSource(int64(int(time.Now().UnixNano()) * os.Getpid())))
	config := parseFlags()
	ttl := time.Duration(float64(config.UpdateInterval) * 1.5)
	cache, err := NewCache(config.CacheURL, config.Id, ttl)
	if err != nil {
		log.Fatal(err)
	}
	docker, err := dockerclient.NewDockerClient(config.DockerURL)
	if err != nil {
		log.Fatal(err)
	}
	rtInfo := &RuntimeInfo{config.Id, cache, docker, ttl}
	log.Printf("Started monitoring Docker events (%s)\n", config.Id)
	docker.StartMonitorEvents(dockerEventCallback, rtInfo)
	go func() {
		// Garbage collect expired hosts at random interval
		for {
			cache.ClearExpiredHosts()
			offset := random.Intn(int(config.UpdateInterval.Seconds()))
			time.Sleep(config.UpdateInterval + (time.Duration(offset) * time.Second))
		}
	}()
	func() {
		for {
			update(rtInfo)
			time.Sleep(config.UpdateInterval)
		}
	}()
}
