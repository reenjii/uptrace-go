module github.com/uptrace/uptrace-go/extra/otelgorm

go 1.17

replace github.com/uptrace/uptrace-go/extra/otelsql => ../otelsql

require (
	github.com/uptrace/uptrace-go/extra/otelsql v1.2.0
	go.opentelemetry.io/otel v1.2.0
	go.opentelemetry.io/otel/trace v1.2.0
	gorm.io/gorm v1.22.3
)

require (
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.2 // indirect
	go.opentelemetry.io/otel/internal/metric v0.25.0 // indirect
	go.opentelemetry.io/otel/metric v0.25.0 // indirect
)
