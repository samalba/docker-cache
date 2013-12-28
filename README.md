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


Only Redis?
-----------

Other cache servers (other than Redis) will come, the Cache API has been absctracted carefully to add
support for more cache servers.
