module github.com/uptrace/uptrace-go/example/beego

go 1.15

replace github.com/uptrace/uptrace-go => ../..

require (
	github.com/astaxie/beego v1.12.3
	github.com/prometheus/common v0.30.0 // indirect
	github.com/prometheus/procfs v0.7.2 // indirect
	github.com/shiena/ansicolor v0.0.0-20200904210342-c7312218db18 // indirect
	github.com/uptrace/uptrace-go v1.0.0-RC3
	go.opentelemetry.io/contrib/instrumentation/github.com/astaxie/beego/otelbeego v0.22.0
	go.opentelemetry.io/otel v1.0.0-RC2
	go.opentelemetry.io/otel/trace v1.0.0-RC2
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
)
