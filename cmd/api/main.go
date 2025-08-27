package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	httpSwagger "github.com/swaggo/http-swagger"
	"go.opentelemetry.io/otel/trace"

	"orderflow/pkg/logger"
	"orderflow/pkg/order"
	pg "orderflow/pkg/order/postgres"
	"orderflow/pkg/otel"
)

var (
	redisClient *redis.Client
	repo        order.Repository
	log         *logger.Logger
	tracer      trace.Tracer
)

// @title OrderFlow API
// @version 1.0
// @description API for managing orders
// @host localhost:8443
// @BasePath /
func main() {
	log = logger.New(os.Stdout, logger.LevelInfo, "orderflow", otel.GetTraceID)
	tp, shutdown, err := otel.InitTracing(log, otel.Config{ServiceName: "orderflow", Host: os.Getenv("OTEL_HOST"), Probability: 1.0})
	if err != nil {
		log.Error(context.Background(), "init tracing", "error", err)
		return
	}
	defer shutdown(context.Background())
	tracer = tp.Tracer("orderflow")

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Error(context.Background(), "db connect", "error", err)
		os.Exit(1)
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS orders (id TEXT PRIMARY KEY, item TEXT, quantity INT)"); err != nil {
		log.Error(context.Background(), "create table", "error", err)
		os.Exit(1)
	}
	repo = pg.New(db)

	redisClient = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR")})

	r := mux.NewRouter()
	r.Use(traceMiddleware)
	r.HandleFunc("/login", loginHandler).Methods(http.MethodPost)

	api := r.PathPrefix("/orders").Subrouter()
	api.Use(authMiddleware)
	api.HandleFunc("", createOrderHandler).Methods(http.MethodPost)
	api.HandleFunc("", listOrdersHandler).Methods(http.MethodGet)
	api.HandleFunc("/{id}", getOrderHandler).Methods(http.MethodGet)
	api.HandleFunc("/{id}", updateOrderHandler).Methods(http.MethodPut)
	api.HandleFunc("/{id}", deleteOrderHandler).Methods(http.MethodDelete)

	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	log.Info(context.Background(), "listening", "addr", ":8443")
	if err := http.ListenAndServeTLS(":8443", "certs/server.crt", "certs/server.key", r); err != nil {
		log.Error(context.Background(), "server closed", "error", err)
	}
}

// loginHandler handles user login and session creation.
// @Summary Login
// @Description Authenticates user and sets session cookie
// @Accept json
// @Produce json
// @Param creds body loginRequest true "Credentials"
// @Success 200
// @Router /login [post]
func loginHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.AddSpan(r.Context(), "loginHandler")
	defer span.End()

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		http.Error(w, "invalid credentials", http.StatusBadRequest)
		return
	}
	sid := strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := redisClient.Set(ctx, "session:"+sid, req.Username, time.Hour).Err(); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session_id", Value: sid, Path: "/", Expires: time.Now().Add(time.Hour), HttpOnly: true})
	w.WriteHeader(http.StatusOK)
}

// authMiddleware ensures a valid session exists.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session_id")
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		user, err := redisClient.Get(r.Context(), "session:"+c.Value).Result()
		if err != nil || user == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// createOrderHandler creates a new order.
// @Summary Create order
// @Accept json
// @Produce json
// @Param order body order.Order true "Order"
// @Success 201 {object} order.Order
// @Security ApiKeyAuth
// @Router /orders [post]
func createOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.AddSpan(r.Context(), "createOrderHandler")
	defer span.End()

	var o order.Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if o.ID == "" {
		o.ID = strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	if err := repo.Create(ctx, o); err != nil {
		log.Error(ctx, "create order", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(o)
}

// listOrdersHandler lists orders.
// @Summary List orders
// @Produce json
// @Success 200 {array} order.Order
// @Security ApiKeyAuth
// @Router /orders [get]
func listOrdersHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.AddSpan(r.Context(), "listOrdersHandler")
	defer span.End()

	orders, err := repo.List(ctx)
	if err != nil {
		log.Error(ctx, "list orders", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

// getOrderHandler retrieves an order by ID.
// @Summary Get order
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} order.Order
// @Security ApiKeyAuth
// @Router /orders/{id} [get]
func getOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.AddSpan(r.Context(), "getOrderHandler")
	defer span.End()

	id := mux.Vars(r)["id"]
	o, err := repo.Get(ctx, id)
	if err != nil {
		if err == order.ErrNotFound {
			http.NotFound(w, r)
			return
		}
		log.Error(ctx, "get order", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(o)
}

// updateOrderHandler updates an existing order.
// @Summary Update order
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param order body order.Order true "Order"
// @Success 200 {object} order.Order
// @Security ApiKeyAuth
// @Router /orders/{id} [put]
func updateOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.AddSpan(r.Context(), "updateOrderHandler")
	defer span.End()

	id := mux.Vars(r)["id"]
	var o order.Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	o.ID = id
	if err := repo.Update(ctx, o); err != nil {
		if err == order.ErrNotFound {
			http.NotFound(w, r)
			return
		}
		log.Error(ctx, "update order", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(o)
}

// deleteOrderHandler removes an order.
// @Summary Delete order
// @Param id path string true "Order ID"
// @Success 204
// @Security ApiKeyAuth
// @Router /orders/{id} [delete]
func deleteOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.AddSpan(r.Context(), "deleteOrderHandler")
	defer span.End()

	id := mux.Vars(r)["id"]
	if err := repo.Delete(ctx, id); err != nil {
		if err == order.ErrNotFound {
			http.NotFound(w, r)
			return
		}
		log.Error(ctx, "delete order", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func traceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.InjectTracing(r.Context(), tracer)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// loginRequest represents login credentials.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
