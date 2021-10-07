module github.com/uptrace/uptrace-go/example/go-redis-cluster

go 1.15

replace github.com/uptrace/uptrace-go => ../..

require (
	github.com/go-redis/redis/extra/redisotel/v8 v8.11.4
	github.com/go-redis/redis/v8 v8.11.4
	github.com/uptrace/uptrace-go v1.0.2
	go.opentelemetry.io/otel v1.0.1
)
