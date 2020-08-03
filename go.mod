module github.com/bruno-anjos/archimedes

go 1.13

require (
	github.com/bruno-anjos/archimedesHTTPClient v0.0.0-20200731165616-9aa4edba78b5
	github.com/bruno-anjos/scheduler v0.0.0-20200730160240-da949fb91af9
	github.com/bruno-anjos/solution-utils v0.0.0-20200731153528-f4f5b5285b7d
	github.com/docker/go-connections v0.4.0
	github.com/gorilla/mux v1.7.4
	github.com/sirupsen/logrus v1.6.0
)

replace (
	github.com/bruno-anjos/archimedesHTTPClient v0.0.0-20200731165616-9aa4edba78b5 => ./../archimedesHTTPClient
	github.com/bruno-anjos/scheduler v0.0.0-20200730160240-da949fb91af9 => ./../scheduler
	github.com/bruno-anjos/solution-utils v0.0.0-20200731153528-f4f5b5285b7d => ./../solution-utils
)
