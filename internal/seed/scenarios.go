package seed

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func (e *emitter) browse(tid pcommon.TraceID, t0 time.Time) int {
	hit := e.rng.Intn(3) != 0
	root := e.span(spanIn{
		tid: tid, service: "web-bff", name: "HTTP GET /products", kind: ptrace.SpanKindServer,
		start: t0, end: t0.Add(ms(42 + jitter(e.rng, 20))), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("GET", "/products", "200"),
	})
	gw := e.span(spanIn{
		tid: tid, service: "gateway", name: "HTTP GET /products", kind: ptrace.SpanKindServer,
		parent: root, start: t0.Add(ms(1)), end: t0.Add(ms(38)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("GET", "/products", "200"),
	})
	cat := e.span(spanIn{
		tid: tid, service: "catalog", name: "ListProducts", kind: ptrace.SpanKindInternal,
		parent: gw, start: t0.Add(ms(3)), end: t0.Add(ms(34)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.system": "grpc", "rpc.method": "ListProducts"},
	})
	redisEnd := t0.Add(ms(8))
	redisCode := ptrace.StatusCodeOk
	var redisEvents []event
	if hit {
		redisEvents = []event{{name: "cache.hit", at: t0.Add(ms(6)), attrs: map[string]string{"cache.key": "products:home"}}}
	} else {
		redisEnd = t0.Add(ms(5))
		redisEvents = []event{{name: "cache.miss", at: t0.Add(ms(4)), attrs: map[string]string{"cache.key": "products:home"}}}
	}
	e.span(spanIn{
		tid: tid, service: "redis", name: "GET products:home", kind: ptrace.SpanKindClient,
		parent: cat, start: t0.Add(ms(4)), end: redisEnd, code: redisCode,
		attrs:  map[string]string{"db.system": "redis", "db.operation": "GET", "db.redis.database_index": "0"},
		events: redisEvents,
	})
	if !hit {
		e.span(spanIn{
			tid: tid, service: "postgres", name: "SELECT products", kind: ptrace.SpanKindClient,
			parent: cat, start: t0.Add(ms(9)), end: t0.Add(ms(28)), code: ptrace.StatusCodeOk,
			attrs: map[string]string{
				"db.system":    "postgresql",
				"db.name":      "shop",
				"db.statement": "SELECT id, name, price FROM products WHERE featured = true LIMIT 24",
			},
		})
	}
	return 1
}

func (e *emitter) search(tid pcommon.TraceID, t0 time.Time) int {
	slow := e.rng.Intn(5) == 0
	q := "linen shirt"
	esEnd := t0.Add(ms(24))
	if slow {
		esEnd = t0.Add(ms(180))
	}
	root := e.span(spanIn{
		tid: tid, service: "web-bff", name: "HTTP GET /search", kind: ptrace.SpanKindServer,
		start: t0, end: esEnd.Add(ms(8)), code: ptrace.StatusCodeOk,
		attrs: merge(httpAttrs("GET", "/search", "200"), map[string]string{"url.query": "q=" + q}),
	})
	gw := e.span(spanIn{
		tid: tid, service: "gateway", name: "HTTP GET /search", kind: ptrace.SpanKindServer,
		parent: root, start: t0.Add(ms(1)), end: esEnd.Add(ms(5)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("GET", "/search", "200"),
	})
	srch := e.span(spanIn{
		tid: tid, service: "search", name: "SearchProducts", kind: ptrace.SpanKindInternal,
		parent: gw, start: t0.Add(ms(3)), end: esEnd.Add(ms(2)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "SearchProducts", "search.query": q},
	})
	e.span(spanIn{
		tid: tid, service: "elasticsearch", name: "POST /products/_search", kind: ptrace.SpanKindClient,
		parent: srch, start: t0.Add(ms(5)), end: esEnd, code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "elasticsearch", "db.operation": "search", "db.elasticsearch.path": "/products/_search"},
	})
	return 1
}

func (e *emitter) login(tid pcommon.TraceID, t0 time.Time) int {
	root := e.span(spanIn{
		tid: tid, service: "web-bff", name: "HTTP POST /login", kind: ptrace.SpanKindServer,
		start: t0, end: t0.Add(ms(55)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("POST", "/login", "200"),
	})
	gw := e.span(spanIn{
		tid: tid, service: "gateway", name: "HTTP POST /login", kind: ptrace.SpanKindServer,
		parent: root, start: t0.Add(ms(1)), end: t0.Add(ms(50)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("POST", "/login", "200"),
	})
	auth := e.span(spanIn{
		tid: tid, service: "auth", name: "Login", kind: ptrace.SpanKindInternal,
		parent: gw, start: t0.Add(ms(3)), end: t0.Add(ms(46)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.system": "grpc", "rpc.method": "Login", "enduser.id": "u-1842"},
	})
	e.span(spanIn{
		tid: tid, service: "postgres", name: "LookupUser", kind: ptrace.SpanKindClient,
		parent: auth, start: t0.Add(ms(5)), end: t0.Add(ms(22)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "postgresql", "db.statement": "SELECT id, hash FROM users WHERE email = $1"},
	})
	e.span(spanIn{
		tid: tid, service: "redis", name: "SET session", kind: ptrace.SpanKindClient,
		parent: auth, start: t0.Add(ms(24)), end: t0.Add(ms(30)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "redis", "db.operation": "SET"},
	})
	return 1
}

func (e *emitter) cart(tid pcommon.TraceID, t0 time.Time) int {
	root := e.span(spanIn{
		tid: tid, service: "web-bff", name: "HTTP POST /cart", kind: ptrace.SpanKindServer,
		start: t0, end: t0.Add(ms(36)), code: ptrace.StatusCodeOk,
		attrs: merge(httpAttrs("POST", "/cart", "200"), map[string]string{"enduser.id": "u-1842"}),
	})
	gw := e.span(spanIn{
		tid: tid, service: "gateway", name: "HTTP POST /cart", kind: ptrace.SpanKindServer,
		parent: root, start: t0.Add(ms(1)), end: t0.Add(ms(32)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("POST", "/cart", "200"),
	})
	c := e.span(spanIn{
		tid: tid, service: "cart", name: "AddItem", kind: ptrace.SpanKindInternal,
		parent: gw, start: t0.Add(ms(3)), end: t0.Add(ms(28)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "AddItem", "product.id": "sku-2291"},
	})
	e.span(spanIn{
		tid: tid, service: "redis", name: "HSET cart", kind: ptrace.SpanKindClient,
		parent: c, start: t0.Add(ms(4)), end: t0.Add(ms(9)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "redis", "db.operation": "HSET"},
	})
	e.span(spanIn{
		tid: tid, service: "postgres", name: "InsertCartItem", kind: ptrace.SpanKindClient,
		parent: c, start: t0.Add(ms(10)), end: t0.Add(ms(22)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "postgresql", "db.statement": "INSERT INTO cart_items (cart_id, sku, qty) VALUES ($1, $2, $3)"},
	})
	return 1
}

func (e *emitter) checkoutOK(tid pcommon.TraceID, t0 time.Time) int {
	root := e.span(spanIn{
		tid: tid, service: "web-bff", name: "HTTP POST /checkout", kind: ptrace.SpanKindServer,
		start: t0, end: t0.Add(ms(96)), code: ptrace.StatusCodeOk,
		attrs: merge(httpAttrs("POST", "/checkout", "200"), map[string]string{"enduser.id": "u-1842"}),
	})
	gw := e.span(spanIn{
		tid: tid, service: "gateway", name: "HTTP POST /checkout", kind: ptrace.SpanKindServer,
		parent: root, start: t0.Add(ms(1)), end: t0.Add(ms(92)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("POST", "/checkout", "200"),
	})
	e.span(spanIn{
		tid: tid, service: "auth", name: "VerifyToken", kind: ptrace.SpanKindInternal,
		parent: gw, start: t0.Add(ms(2)), end: t0.Add(ms(8)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.system": "grpc", "rpc.method": "VerifyToken"},
	})
	pay := e.span(spanIn{
		tid: tid, service: "checkout", name: "HTTP POST /pay", kind: ptrace.SpanKindServer,
		parent: gw, start: t0.Add(ms(10)), end: t0.Add(ms(88)), code: ptrace.StatusCodeOk,
		attrs:  merge(httpAttrs("POST", "/pay", "200"), map[string]string{"order.id": fmt.Sprintf("ord-%04d", e.rng.Intn(9000)+1000)}),
		events: []event{{name: "checkout.step", at: t0.Add(ms(12)), attrs: map[string]string{"step": "charge"}}},
	})
	e.span(spanIn{
		tid: tid, service: "redis", name: "GetCart", kind: ptrace.SpanKindClient,
		parent: pay, start: t0.Add(ms(12)), end: t0.Add(ms(16)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "redis", "db.operation": "GET"},
	})
	e.span(spanIn{
		tid: tid, service: "postgres", name: "QueryOrder", kind: ptrace.SpanKindClient,
		parent: pay, start: t0.Add(ms(14)), end: t0.Add(ms(32)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "postgresql", "db.statement": "SELECT * FROM orders WHERE id = $1"},
	})
	inv := e.span(spanIn{
		tid: tid, service: "inventory", name: "Reserve", kind: ptrace.SpanKindServer,
		parent: pay, start: t0.Add(ms(18)), end: t0.Add(ms(40)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "Reserve"},
	})
	e.span(spanIn{
		tid: tid, service: "postgres", name: "UpdateStock", kind: ptrace.SpanKindClient,
		parent: inv, start: t0.Add(ms(20)), end: t0.Add(ms(36)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "postgresql", "db.statement": "UPDATE stock SET qty = qty - $1 WHERE sku = $2"},
	})
	chg := e.span(spanIn{
		tid: tid, service: "payment", name: "Charge", kind: ptrace.SpanKindServer,
		parent: pay, start: t0.Add(ms(22)), end: t0.Add(ms(70)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "Charge", "payment.method": "card"},
	})
	e.span(spanIn{
		tid: tid, service: "fraud", name: "Score", kind: ptrace.SpanKindClient,
		parent: chg, start: t0.Add(ms(24)), end: t0.Add(ms(38)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.system": "grpc", "rpc.method": "Score"},
	})
	e.span(spanIn{
		tid: tid, service: "postgres", name: "InsertPayment", kind: ptrace.SpanKindClient,
		parent: chg, start: t0.Add(ms(40)), end: t0.Add(ms(55)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "postgresql", "db.statement": "INSERT INTO payments (order_id, amount) VALUES ($1, $2)"},
	})
	prod := e.span(spanIn{
		tid: tid, service: "checkout", name: "publish orders.confirmed", kind: ptrace.SpanKindProducer,
		parent: pay, start: t0.Add(ms(72)), end: t0.Add(ms(78)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"messaging.system": "kafka", "messaging.destination": "orders.confirmed", "messaging.operation": "publish"},
	})
	e.span(spanIn{
		tid: tid, service: "kafka", name: "append orders.confirmed", kind: ptrace.SpanKindInternal,
		parent: prod, start: t0.Add(ms(73)), end: t0.Add(ms(77)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"messaging.system": "kafka", "messaging.destination": "orders.confirmed"},
	})
	return 1
}

func (e *emitter) checkoutAuthFail(tid pcommon.TraceID, t0 time.Time) int {
	root := e.span(spanIn{
		tid: tid, service: "web-bff", name: "HTTP POST /checkout", kind: ptrace.SpanKindServer,
		start: t0, end: t0.Add(ms(28)), code: ptrace.StatusCodeError, msg: "unauthorized",
		attrs: httpAttrs("POST", "/checkout", "401"),
	})
	gw := e.span(spanIn{
		tid: tid, service: "gateway", name: "HTTP POST /checkout", kind: ptrace.SpanKindServer,
		parent: root, start: t0.Add(ms(1)), end: t0.Add(ms(24)), code: ptrace.StatusCodeError, msg: "unauthorized",
		attrs: httpAttrs("POST", "/checkout", "401"),
	})
	e.span(spanIn{
		tid: tid, service: "auth", name: "VerifyToken", kind: ptrace.SpanKindInternal,
		parent: gw, start: t0.Add(ms(2)), end: t0.Add(ms(18)), code: ptrace.StatusCodeError, msg: "token expired",
		attrs: map[string]string{"rpc.system": "grpc", "rpc.method": "VerifyToken"},
		events: []event{{
			name: "exception",
			at:   t0.Add(ms(12)),
			attrs: map[string]string{
				"exception.type":    "AuthError",
				"exception.message": "token expired",
			},
		}},
	})
	return 1
}

func (e *emitter) checkoutPayFail(tid pcommon.TraceID, t0 time.Time) int {
	root := e.span(spanIn{
		tid: tid, service: "web-bff", name: "HTTP POST /checkout", kind: ptrace.SpanKindServer,
		start: t0, end: t0.Add(ms(64)), code: ptrace.StatusCodeError, msg: "payment declined",
		attrs: httpAttrs("POST", "/checkout", "402"),
	})
	gw := e.span(spanIn{
		tid: tid, service: "gateway", name: "HTTP POST /checkout", kind: ptrace.SpanKindServer,
		parent: root, start: t0.Add(ms(1)), end: t0.Add(ms(60)), code: ptrace.StatusCodeError, msg: "payment declined",
		attrs: httpAttrs("POST", "/checkout", "402"),
	})
	e.span(spanIn{
		tid: tid, service: "auth", name: "VerifyToken", kind: ptrace.SpanKindInternal,
		parent: gw, start: t0.Add(ms(2)), end: t0.Add(ms(7)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "VerifyToken"},
	})
	pay := e.span(spanIn{
		tid: tid, service: "checkout", name: "HTTP POST /pay", kind: ptrace.SpanKindServer,
		parent: gw, start: t0.Add(ms(9)), end: t0.Add(ms(56)), code: ptrace.StatusCodeError, msg: "card declined",
		attrs: merge(httpAttrs("POST", "/pay", "402"), map[string]string{"order.id": "ord-fail"}),
	})
	e.span(spanIn{
		tid: tid, service: "redis", name: "GetCart", kind: ptrace.SpanKindClient,
		parent: pay, start: t0.Add(ms(11)), end: t0.Add(ms(14)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "redis", "db.operation": "GET"},
	})
	chg := e.span(spanIn{
		tid: tid, service: "payment", name: "Charge", kind: ptrace.SpanKindServer,
		parent: pay, start: t0.Add(ms(16)), end: t0.Add(ms(48)), code: ptrace.StatusCodeError, msg: "insufficient funds",
		attrs: map[string]string{"rpc.method": "Charge", "http.status_code": "402"},
		events: []event{{
			name: "exception",
			at:   t0.Add(ms(40)),
			attrs: map[string]string{
				"exception.type":    "CardDeclined",
				"exception.message": "insufficient funds",
			},
		}},
	})
	e.span(spanIn{
		tid: tid, service: "fraud", name: "Score", kind: ptrace.SpanKindClient,
		parent: chg, start: t0.Add(ms(18)), end: t0.Add(ms(28)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "Score"},
	})
	return 1
}

func (e *emitter) checkoutSlow(tid pcommon.TraceID, t0 time.Time) int {
	root := e.span(spanIn{
		tid: tid, service: "web-bff", name: "HTTP POST /checkout", kind: ptrace.SpanKindServer,
		start: t0, end: t0.Add(ms(520)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("POST", "/checkout", "200"),
	})
	gw := e.span(spanIn{
		tid: tid, service: "gateway", name: "HTTP POST /checkout", kind: ptrace.SpanKindServer,
		parent: root, start: t0.Add(ms(1)), end: t0.Add(ms(510)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("POST", "/checkout", "200"),
	})
	pay := e.span(spanIn{
		tid: tid, service: "checkout", name: "HTTP POST /pay", kind: ptrace.SpanKindServer,
		parent: gw, start: t0.Add(ms(4)), end: t0.Add(ms(500)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("POST", "/pay", "200"),
	})
	e.span(spanIn{
		tid: tid, service: "postgres", name: "QueryOrder", kind: ptrace.SpanKindClient,
		parent: pay, start: t0.Add(ms(8)), end: t0.Add(ms(430)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{
			"db.system":    "postgresql",
			"db.statement": "SELECT * FROM orders o JOIN order_items i ON i.order_id = o.id WHERE o.id = $1",
		},
		events: []event{{name: "db.query.plan", at: t0.Add(ms(200)), attrs: map[string]string{"hint": "seq scan"}}},
	})
	return 1
}

func (e *emitter) fanout(tid pcommon.TraceID, t0 time.Time) int {
	root := e.span(spanIn{
		tid: tid, service: "web-bff", name: "HTTP GET /home", kind: ptrace.SpanKindServer,
		start: t0, end: t0.Add(ms(48)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("GET", "/home", "200"),
	})
	gw := e.span(spanIn{
		tid: tid, service: "gateway", name: "HTTP GET /home", kind: ptrace.SpanKindServer,
		parent: root, start: t0.Add(ms(1)), end: t0.Add(ms(44)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("GET", "/home", "200"),
	})
	e.span(spanIn{
		tid: tid, service: "auth", name: "Session", kind: ptrace.SpanKindInternal,
		parent: gw, start: t0.Add(ms(2)), end: t0.Add(ms(9)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "Session"},
	})
	e.span(spanIn{
		tid: tid, service: "catalog", name: "ListProducts", kind: ptrace.SpanKindInternal,
		parent: gw, start: t0.Add(ms(2)), end: t0.Add(ms(22)), code: ptrace.StatusCodeUnset,
		attrs: map[string]string{"rpc.method": "ListProducts"},
	})
	e.span(spanIn{
		tid: tid, service: "cart", name: "GetCart", kind: ptrace.SpanKindInternal,
		parent: gw, start: t0.Add(ms(3)), end: t0.Add(ms(14)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "GetCart"},
	})
	e.span(spanIn{
		tid: tid, service: "search", name: "Popular", kind: ptrace.SpanKindInternal,
		parent: gw, start: t0.Add(ms(3)), end: t0.Add(ms(30)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "Popular"},
	})
	return 1
}

func (e *emitter) deep(tid pcommon.TraceID, t0 time.Time) int {
	a := e.span(spanIn{
		tid: tid, service: "web-bff", name: "HTTP GET /orders/latest", kind: ptrace.SpanKindServer,
		start: t0, end: t0.Add(ms(70)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("GET", "/orders/latest", "200"),
	})
	b := e.span(spanIn{
		tid: tid, service: "gateway", name: "HTTP GET /orders/latest", kind: ptrace.SpanKindServer,
		parent: a, start: t0.Add(ms(1)), end: t0.Add(ms(66)), code: ptrace.StatusCodeOk,
		attrs: httpAttrs("GET", "/orders/latest", "200"),
	})
	c := e.span(spanIn{
		tid: tid, service: "checkout", name: "GetLatestOrder", kind: ptrace.SpanKindInternal,
		parent: b, start: t0.Add(ms(3)), end: t0.Add(ms(60)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "GetLatestOrder"},
	})
	d := e.span(spanIn{
		tid: tid, service: "inventory", name: "GetAvailability", kind: ptrace.SpanKindServer,
		parent: c, start: t0.Add(ms(6)), end: t0.Add(ms(48)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "GetAvailability"},
	})
	f := e.span(spanIn{
		tid: tid, service: "catalog", name: "HydrateSKU", kind: ptrace.SpanKindInternal,
		parent: d, start: t0.Add(ms(10)), end: t0.Add(ms(40)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"rpc.method": "HydrateSKU"},
	})
	g := e.span(spanIn{
		tid: tid, service: "postgres", name: "SELECT sku", kind: ptrace.SpanKindClient,
		parent: f, start: t0.Add(ms(12)), end: t0.Add(ms(32)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "postgresql", "db.statement": "SELECT * FROM sku WHERE id = $1"},
	})
	e.span(spanIn{
		tid: tid, service: "postgres", name: "ParseRow", kind: ptrace.SpanKindInternal,
		parent: g, start: t0.Add(ms(28)), end: t0.Add(ms(31)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "postgresql"},
	})
	return 1
}

// notifyPair emits a producer trace and a linked consumer trace (2 traces).
func (e *emitter) notifyPair(t0 time.Time) int {
	prodTrace := randTrace(e.rng)
	consTrace := randTrace(e.rng)
	root := e.span(spanIn{
		tid: prodTrace, service: "checkout", name: "ConfirmOrder", kind: ptrace.SpanKindInternal,
		start: t0, end: t0.Add(ms(24)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"order.id": "ord-async"},
	})
	prod := e.span(spanIn{
		tid: prodTrace, service: "checkout", name: "publish orders.confirmed", kind: ptrace.SpanKindProducer,
		parent: root, start: t0.Add(ms(8)), end: t0.Add(ms(16)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"messaging.system": "kafka", "messaging.destination": "orders.confirmed"},
	})
	e.span(spanIn{
		tid: prodTrace, service: "kafka", name: "append orders.confirmed", kind: ptrace.SpanKindInternal,
		parent: prod, start: t0.Add(ms(9)), end: t0.Add(ms(15)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"messaging.system": "kafka", "messaging.destination": "orders.confirmed"},
	})

	consStart := t0.Add(ms(40))
	cons := e.span(spanIn{
		tid: consTrace, service: "notify-worker", name: "ProcessOrder", kind: ptrace.SpanKindConsumer,
		start: consStart, end: consStart.Add(ms(30)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"messaging.system": "kafka", "messaging.destination": "orders.confirmed", "messaging.operation": "receive"},
		links: []link{{trace: prodTrace, span: prod, attrs: map[string]string{"link.type": "follows_from"}}},
	})
	e.span(spanIn{
		tid: consTrace, service: "postgres", name: "InsertNotification", kind: ptrace.SpanKindClient,
		parent: cons, start: consStart.Add(ms(4)), end: consStart.Add(ms(18)), code: ptrace.StatusCodeOk,
		attrs: map[string]string{"db.system": "postgresql", "db.statement": "INSERT INTO notifications (user_id, kind) VALUES ($1, $2)"},
	})
	return 2
}

func httpAttrs(method, route, status string) map[string]string {
	return map[string]string{
		"http.method":      method,
		"http.route":       route,
		"http.status_code": status,
		"http.target":      route,
		"net.peer.name":    "shop.internal",
	}
}

func merge(a, b map[string]string) map[string]string {
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
