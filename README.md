# go-http-proxy

Simple HTTP proxy

## Usage

```golang
    import (
        proxy "github.com/trorg/go-http-proxy"
        "net/http"
        "context"
        "log"
    )

    server := proxy.NewServer("http://127.0.0.1:8080", 1)
    upstream := proxy.NewUpstream()
    upstream.AddServer(server)

    proxy := proxy.NewProxy(upstream)

    // Add before middleware
    proxy.RegisterBeforeHandler(func (next http.Handler) http.Handler {
        // This method will be executed before proxying
        return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
            // Pass variable to other underlying handlers
            ctx := context.WithValue(r.Context(), "myvalue", "1")
            newRequest := r.WithContext(ctx)

            next.ServeHTTP(w, newRequest)
        })
    })

    // Add before middleware
    proxy.RegisterBeforeHandler(func (next http.Handler) http.Handler {
        // This method will be executed before proxying
        return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
            myvalue, ok := r.Context().Value("myvalue").(string)
            if !ok {
                // "myvalue" variable is absent in request's context
                log.Println("can't get myvalue from context")
                return
            }
            r.Header.Set("X-My-Header", myvalue)

            next.ServeHTTP(w, r)
        })
    })

    server := &http.Server{
        Addr: "127.0.0.1:8000",
        Handler: proxy.GetHandler(),
    }

    log.Fatal(server.ListenAndServe())
```
