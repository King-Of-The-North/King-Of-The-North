package httpapi

import "net/http"

// CORS wraps a handler with permissive cross-origin headers so the browser-based
// merchant/admin web app (Next.js) can call the Gateway. origin is the allowed origin
// ("*" for the demo, or a specific http://host:port). Preflight OPTIONS short-circuits
// with 204. Mobile (native fetch) doesn't need this; the web frontend does.
func CORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Vary", "Origin")
		h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		h.Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
