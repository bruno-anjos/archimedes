module github.com/bruno-anjos/archimedes

go 1.13

require (
	github.com/bruno-anjos/scheduler v0.0.0-20200730160240-da949fb91af9
	github.com/bruno-anjos/solution-utils v0.0.0-20200730160126-4bddd1152617
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.4.0 // indirect
	github.com/gorilla/mux v1.7.4
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/sirupsen/logrus v1.6.0
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
)

replace (
	github.com/bruno-anjos/scheduler v0.0.0-20200730160240-da949fb91af9 => ./../scheduler
	github.com/bruno-anjos/solution-utils v0.0.0-20200730160126-4bddd1152617 => ./../solution-utils
)
