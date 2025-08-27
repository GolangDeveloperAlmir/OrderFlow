package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
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

	"orderflow/order"
)

var (
	redisClient *redis.Client
	store       *order.PGStore
)

// @title OrderFlow API
// @version 1.0
// @description API for managing orders
// @host localhost:8443
// @BasePath /
func main() {
	exp, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())
	// Setup Postgres
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS orders (id TEXT PRIMARY KEY, item TEXT, quantity INT)"); err != nil {
		log.Fatalf("create table: %v", err)
	}
	store = order.NewPGStore(db)

	// Setup Redis for sessions
	redisClient = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR")})

	r := mux.NewRouter()
	r.HandleFunc("/login", loginHandler).Methods(http.MethodPost)

	api := r.PathPrefix("/orders").Subrouter()
	api.Use(authMiddleware)
	api.HandleFunc("", createOrderHandler).Methods(http.MethodPost)
	api.HandleFunc("", listOrdersHandler).Methods(http.MethodGet)

	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	log.Println("listening on https://0.0.0.0:8443")
	log.Fatal(http.ListenAndServeTLS(":8443", "certs/server.crt", "certs/server.key", r))
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
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

// createOrderHandler creates a new order
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
	if err := store.Create(r.Context(), o); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(o)
}

// listOrdersHandler lists orders
// @Summary List orders
// @Produce json
// @Success 200 {array} order.Order
// @Security ApiKeyAuth
// @Router /orders [get]
func listOrdersHandler(w http.ResponseWriter, r *http.Request) {
	orders, err := store.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}
