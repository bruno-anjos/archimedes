module github.com/bruno-anjos/archimedes

go 1.13

require (
	github.com/bruno-anjos/scheduler v0.0.0-20200730160240-da949fb91af9
	github.com/bruno-anjos/solution-utils v0.0.0-20200803160423-4cf841cde3d3
	github.com/docker/go-connections v0.4.0
	github.com/gorilla/mux v1.7.4
	github.com/sirupsen/logrus v1.6.0
	golang.org/x/sys v0.0.0-20200323222414-85ca7c5b95cd // indirect
)

replace (
	github.com/bruno-anjos/scheduler v0.0.0-20200730160240-da949fb91af9 => ./../scheduler
	github.com/bruno-anjos/solution-utils v0.0.0-20200803160206-562c9f14e46c => ./../solution-utils
)
