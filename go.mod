module github.com/bruno-anjos/archimedes

go 1.13

require (
	github.com/bruno-anjos/scheduler v0.0.0-20200804151330-da4916073155
	github.com/bruno-anjos/solution-utils v0.0.0-20200804140242-989a419bda22
	github.com/docker/go-connections v0.4.0
	github.com/gorilla/mux v1.7.4
	github.com/sirupsen/logrus v1.6.0
)

replace (
	github.com/bruno-anjos/scheduler v0.0.0-20200804151330-da4916073155 => ./../scheduler
	github.com/bruno-anjos/solution-utils v0.0.0-20200804140242-989a419bda22 => ./../solution-utils
)
