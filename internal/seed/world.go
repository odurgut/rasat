package seed

// Shop is a k8s commerce stack used by the synthetic traces. Names are stable
// so search, the waterfall, the inspector, and the service map all have
// something real to show: HTTP, RPC, DB, cache, search, messaging, errors.
type serviceMeta struct {
	name      string
	version   string
	namespace string
	lang      string
	scope     string
	scopeVer  string
}

func lookupService(name string) serviceMeta {
	if m, ok := services[name]; ok {
		return m
	}
	return serviceMeta{
		name:      name,
		version:   "0.1.0",
		namespace: "shop",
		lang:      "go",
		scope:     "rasat/seed",
		scopeVer:  "0.2.0",
	}
}

var services = map[string]serviceMeta{
	"web-bff": {
		name: "web-bff", version: "3.4.1", namespace: "shop", lang: "javascript",
		scope: "@opentelemetry/instrumentation-http", scopeVer: "0.57.0",
	},
	"gateway": {
		name: "gateway", version: "1.18.0", namespace: "shop", lang: "go",
		scope: "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp", scopeVer: "0.59.0",
	},
	"auth": {
		name: "auth", version: "2.9.3", namespace: "shop", lang: "go",
		scope: "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc", scopeVer: "0.59.0",
	},
	"catalog": {
		name: "catalog", version: "1.12.0", namespace: "shop", lang: "go",
		scope: "go.opentelemetry.io/otel", scopeVer: "1.34.0",
	},
	"cart": {
		name: "cart", version: "1.7.2", namespace: "shop", lang: "go",
		scope: "go.opentelemetry.io/otel", scopeVer: "1.34.0",
	},
	"checkout": {
		name: "checkout", version: "4.2.0", namespace: "shop", lang: "go",
		scope: "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp", scopeVer: "0.59.0",
	},
	"payment": {
		name: "payment", version: "2.1.4", namespace: "shop", lang: "java",
		scope: "io.opentelemetry.jdbc", scopeVer: "2.12.0",
	},
	"inventory": {
		name: "inventory", version: "1.5.0", namespace: "shop", lang: "go",
		scope: "go.opentelemetry.io/otel", scopeVer: "1.34.0",
	},
	"search": {
		name: "search", version: "0.9.8", namespace: "shop", lang: "go",
		scope: "go.opentelemetry.io/otel", scopeVer: "1.34.0",
	},
	"notify-worker": {
		name: "notify-worker", version: "1.3.1", namespace: "shop", lang: "go",
		scope: "go.opentelemetry.io/contrib/instrumentation/github.com/segmentio/kafka-go/otelkafka", scopeVer: "0.59.0",
	},
	"fraud": {
		name: "fraud", version: "0.8.0", namespace: "shop", lang: "python",
		scope: "opentelemetry.instrumentation.grpc", scopeVer: "0.50b0",
	},
	"postgres": {
		name: "postgres", version: "16.3", namespace: "data", lang: "c",
		scope: "rasat/seed/postgres", scopeVer: "0.2.0",
	},
	"redis": {
		name: "redis", version: "7.4.0", namespace: "data", lang: "c",
		scope: "rasat/seed/redis", scopeVer: "0.2.0",
	},
	"elasticsearch": {
		name: "elasticsearch", version: "8.15.0", namespace: "data", lang: "java",
		scope: "rasat/seed/elasticsearch", scopeVer: "0.2.0",
	},
	"kafka": {
		name: "kafka", version: "3.8.0", namespace: "data", lang: "java",
		scope: "rasat/seed/kafka", scopeVer: "0.2.0",
	},
}
