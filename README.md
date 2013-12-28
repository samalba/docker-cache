docker-cache
============

Docker-Cache monitor Docker containers on a given Docker host and report data back to a central Redis.


Why?
----

Docker-Cache brings full visibility on what is happening on your cluster in real-time:

- How many Docker hosts are running?
- How many containers on each of them?
- What is the configuration for each container?

Those data are updated in realtime and you can access them quickly in one location (Redis).

The Cache server is volatile, you can lose the dataset, the data will be repopulated quickly as long as
there is a Cache server running.


How to use it?
--------------

### Launch it

Docker-Cache is configurable through the command-line arguments:

```
$ ./docker-cache -h
Usage of ./docker-cache:
  -cache="redis://localhost:6379": Cache URL which will receive the data
  -docker="unix:///var/run/docker.sock": Docker daemon URL to read the data
  -id="ubuntu": Id that will identify this host on the Cache (it should be unique to avoid conflicts)
  -updateInterval=2m0s: Interval for polling data from Docker
```

No arguments are needed if you use it locally:

```
$ ./docker-cache
2013/12/27 16:30:32 Started monitoring Docker events (ubuntu)
2013/12/27 16:30:32 Reported 4 containers to the Cache
```

### Now use any Redis client to get the data

List the reporting Docker hosts:

```
$ redis-cli smembers docker:hosts
1) "ubuntu"
```

Get some info about the host named "ubuntu":

```
$ redis-cli hget docker:hosts:ubuntu containers_running
"4"

$ redis-cli hget docker:hosts:ubuntu docker_version
"0.7.1;git-88df052;go1.2"
```

List containers name on the host:

```
$ redis-cli smembers docker:hosts:ubuntu:containers
1) "a5e3a42131902cfed61974e615cb8bc1d35e42f15d3b012360f130baf1201f17"
2) "e486d60c14d0fc9a82e098075ca3565771de32edc2054c204b763045d18a060a"
3) "aefdbb4b96a656d2f316926252adc2d968cabf9c78af5f9508381d02c0f7c1ff"
4) "d0686cb2818c142d669ae2cc8222d639fbf21dd3db592f9fb52a4fcc57149c30"
```

Get some info about the first container listed above:

```
$ redis-cli hget docker:containers:a5e3a42131902cfed61974e615cb8bc1d35e42f15d3b012360f130baf1201f17 networksettings_ipaddress
"172.17.0.3"

$ redis-cli hget docker:containers:a5e3a42131902cfed61974e615cb8bc1d35e42f15d3b012360f130baf1201f17 config_cmd
"[\"bash\"]"

$ redis-cli hget docker:containers:a5e3a42131902cfed61974e615cb8bc1d35e42f15d3b012360f130baf1201f17 config_hostname
"a5e3a4213190"

$ redis-cli hget docker:containers:a5e3a42131902cfed61974e615cb8bc1d35e42f15d3b012360f130baf1201f17 config_image
"ubuntu"
```

Or even the raw json of the container:

```
$ redis-cli get docker:containers:a5e3a42131902cfed61974e615cb8bc1d35e42f15d3b012360f130baf1201f17:json
"{\"Id\":\"a5e3a42131902cfed61974e615cb8bc1d35e42f15d3b012360f130baf1201f17\",\"Create\":\"\",\"Path\":\"bash\",\"Args\":[],\"Config\":{\"Hostname\":\"a5e3a4213190\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":true,\"OpenStdin\":true,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"bash\"],\"Dns\":null,\"Image\":\"ubuntu\",\"VolumesFrom\":\"\",\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false},\"State\":{\"Running\":true,\"Pid\":29268,\"ExitCode\":0,\"StartedAt\":\"2013-12-23T23:30:20.794486423Z\",\"Ghost\":false},\"Image\":\"8dbd9e392a964056420e5d58ca5cc376ef18e2de93b5cc90e868a1bbc8318c1c\",\"NetworkSettings\":{\"IpAddress\":\"172.17.0.3\",\"IpPrefixLen\":16,\"Gateway\":\"172.17.42.1\",\"Bridge\":\"docker0\",\"Ports\":{}},\"SysInitPath\":\"/usr/bin/docker\",\"ResolvConfPath\":\"/etc/resolv.conf\",\"Volumes\":{},\"HostConfig\":{\"Binds\":null,\"ContainerIDFile\":\"\",\"LxcConf\":[],\"Privileged\":false,\"PortBindings\":{},\"Links\":null,\"PublishAllPorts\":false}}"
```

Subscribe to real-time changes (below the event corresponds to a new created container):

```
$ redis-cli subscribe docker_events
1) "message"
2) "docker_events"
3) "new_container:ubuntu:f8d6295ab7c54b2ec408dc4b2038c014dd82eac74d31e6af711c5378d2644bf4"
```


Only Redis?
-----------

Other cache servers (other than Redis) will come, the Cache API has been absctracted carefully to add
support for more cache servers.
