module lina-plugin-idc-compliance

go 1.25.0

require (
	github.com/gogf/gf/contrib/drivers/pgsql/v2 v2.10.0
	github.com/gogf/gf/v2 v2.10.1
	github.com/pdfcpu/pdfcpu v0.12.1
	golang.org/x/net v0.52.0
	golang.org/x/text v0.40.0
	lina-core v0.0.0
	lina-platform/idc-compliance-gateway v0.0.0
	lina-plugin-idc-core v0.1.0
	lina-plugin-idc-notify v0.1.0
)

require (
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/clbanning/mxj/v2 v2.7.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/emirpasic/gods/v2 v2.0.0-alpha // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/gowebpki/jcs v1.0.1 // indirect
	github.com/grokify/html-strip-tags-go v0.1.0 // indirect
	github.com/hhrutter/lzw v1.0.0 // indirect
	github.com/hhrutter/pkcs7 v0.2.2 // indirect
	github.com/hhrutter/tiff v1.0.3 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.23 // indirect
	github.com/olekukonko/errors v1.1.0 // indirect
	github.com/olekukonko/ll v0.0.9 // indirect
	github.com/olekukonko/tablewriter v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/image v0.39.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lina-plugin-shared-services v0.1.0 // indirect
)

replace lina-core => ../../lina-core

replace lina-plugin-idc-core => ../idc-core

replace lina-plugin-idc-notify => ../idc-notify

replace lina-plugin-shared-services => ../shared-services

replace lina-platform/idc-compliance-gateway => ./central-gateway
