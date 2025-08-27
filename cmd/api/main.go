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
	"go.opentelemetry.io/otel"
	stdouttrace "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"

	"orderflow/pkg/logger"
	"orderflow/pkg/order"
	pg "orderflow/pkg/order/postgres"
)

var (
	redisClient *redis.Client
	repo        order.Repository
)

// @title OrderFlow API
// @version 1.0
// @description API for managing orders
// @host localhost:8443
// @BasePath /
func main() {
	logger.Init()
	defer logger.Log.Sync()

	exp, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Log.Fatal("db connect", zap.Error(err))
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS orders (id TEXT PRIMARY KEY, item TEXT, quantity INT)"); err != nil {
		logger.Log.Fatal("create table", zap.Error(err))
	}
	repo = pg.New(db)

	redisClient = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR")})

	r := mux.NewRouter()
	r.HandleFunc("/login", loginHandler).Methods(http.MethodPost)

	api := r.PathPrefix("/orders").Subrouter()
	api.Use(authMiddleware)
	api.HandleFunc("", createOrderHandler).Methods(http.MethodPost)
	api.HandleFunc("", listOrdersHandler).Methods(http.MethodGet)
	api.HandleFunc("/{id}", getOrderHandler).Methods(http.MethodGet)
	api.HandleFunc("/{id}", updateOrderHandler).Methods(http.MethodPut)
	api.HandleFunc("/{id}", deleteOrderHandler).Methods(http.MethodDelete)

	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	logger.Log.Info("listening", zap.String("addr", ":8443"))
	logger.Log.Fatal("server closed", zap.Error(http.ListenAndServeTLS(":8443", "certs/server.crt", "certs/server.key", r)))
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
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		http.Error(w, "invalid credentials", http.StatusBadRequest)
		return
	}
	sid := strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := redisClient.Set(r.Context(), "session:"+sid, req.Username, time.Hour).Err(); err != nil {
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
	var o order.Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if o.ID == "" {
		o.ID = strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	if err := repo.Create(r.Context(), o); err != nil {
		logger.Log.Error("create order", zap.Error(err))
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
	orders, err := repo.List(r.Context())
	if err != nil {
		logger.Log.Error("list orders", zap.Error(err))
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
	id := mux.Vars(r)["id"]
	o, err := repo.Get(r.Context(), id)
	if err != nil {
		if err == order.ErrNotFound {
			http.NotFound(w, r)
			return
		}
		logger.Log.Error("get order", zap.Error(err))
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
	id := mux.Vars(r)["id"]
	var o order.Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	o.ID = id
	if err := repo.Update(r.Context(), o); err != nil {
		if err == order.ErrNotFound {
			http.NotFound(w, r)
			return
		}
		logger.Log.Error("update order", zap.Error(err))
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
	id := mux.Vars(r)["id"]
	if err := repo.Delete(r.Context(), id); err != nil {
		if err == order.ErrNotFound {
			http.NotFound(w, r)
			return
		}
		logger.Log.Error("delete order", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// loginRequest represents login credentials.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
