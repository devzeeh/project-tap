package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"project-tap/internal/admin"
	"project-tap/internal/auth"
	"project-tap/internal/merchant"
	"project-tap/internal/middleware"
	"project-tap/internal/pkg/database"
	"project-tap/internal/pkg/storage"
	"project-tap/internal/user"

	"github.com/joho/godotenv"
)

var (
	tpl *template.Template
)

func main() {
	// Load .env file
	err := godotenv.Load("./.env")
	if err != nil {
		// Fallback: try loading from current directory
		if err := godotenv.Load(); err != nil {
			log.Fatalf("Error loading .env file: %v", err)
		}
	}

	// read .env VALUES
	port := os.Getenv("PORT")
	serverAddress := os.Getenv("SERVER_PORT")

	// Setup Database using the new database package
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	store := database.NewStore(db)

	// Initialize R2 Storage
	r2Storage, err := storage.NewR2Storage()
	if err != nil {
		log.Printf("Warning: Failed to initialize R2 storage (uploads may fail): %v", err)
	}

	// Initialize the Handler from the auth package
	authRepo := auth.NewRepository(store)
	authSvc := auth.NewService(authRepo, r2Storage)
	authHandler := auth.NewHandler(authSvc, tpl)

	adminRepo := admin.NewRepository(store)
	adminSvc := admin.NewService(adminRepo)
	adminHanlder := admin.NewHandler(adminSvc, tpl)

	merchantRepo := merchant.NewRepository(store)
	payoutGateway := merchant.NewXenditPayoutGateway(os.Getenv("XENDIT_SECRET_KEY"))
	merchantSvc := merchant.NewService(merchantRepo, r2Storage, payoutGateway)
	merchantHandler := merchant.NewHandler(merchantSvc, tpl)

	userHandler := user.NewHandler(store, tpl)

	// Middleware definitions
	requireCustomer := middleware.RequireAuth("customer")
	requireMerchant := middleware.RequireAuth("merchant_admin", "merchant_staff")
	requireAdmin := middleware.RequireAuth("super_admin")

	// create mux
	mux := http.NewServeMux()
	// register routes
	auth.RegisterRoutes(mux, authHandler)

	// Routes admin endpoints
	admin.RegisterRoutes(mux, adminHanlder, requireAdmin)
	merchant.RegisterRoutes(mux, merchantHandler, requireMerchant)

	// Customer Routes
	mux.Handle("GET /u/{username}", requireCustomer(http.HandlerFunc(userHandler.ProfileView)))
	mux.Handle("PATCH /u/{username}/profile/edit", requireCustomer(http.HandlerFunc(userHandler.ProfileEdit)))
	mux.Handle("POST /v1/user/{username}/profile/verify-password", requireCustomer(http.HandlerFunc(userHandler.ProfileVerifyPassword)))
	mux.Handle("PUT /u/{username}/profile/password", requireCustomer(http.HandlerFunc(userHandler.ProfileChangePassword)))
	mux.Handle("GET /u/{username}/dashboard", requireCustomer(http.HandlerFunc(userHandler.DashboardView)))
	mux.Handle("GET /u/{username}/card", requireCustomer(http.HandlerFunc(userHandler.CardView)))
	mux.Handle("POST /v1/user/{username}/card/status", requireCustomer(http.HandlerFunc(userHandler.UpdateCardStatus)))
	mux.Handle("POST /v1/user/{username}/card/replace", requireCustomer(http.HandlerFunc(userHandler.RequestReplacement)))
	mux.Handle("GET /u/{username}/settings", requireCustomer(http.HandlerFunc(userHandler.SettingsView)))
	mux.Handle("GET /u/{username}/topup", requireCustomer(http.HandlerFunc(userHandler.TopUpView)))
	// Your frontend calls this to get the Xendit URL
	mux.Handle("POST /api/topup/create-session/{username}", requireCustomer(http.HandlerFunc(userHandler.CreateXenditInvoice)))

	// Payment gateway endpoints
	// Xendit's servers call this behind the scenes when the payment is done
	mux.HandleFunc("POST /api/webhooks/xendit/invoice", userHandler.XenditWebhook)
	mux.Handle("POST /v1/user/{username}/topup/checkout", requireCustomer(http.HandlerFunc(userHandler.CreateXenditInvoice)))
	mux.Handle("GET /u/{username}/transaction", requireCustomer(http.HandlerFunc(userHandler.TransactionView)))
	mux.Handle("GET /u/{username}/transactions", requireCustomer(http.HandlerFunc(userHandler.TransactionView)))

	mux.Handle("GET /v1/user/{username}", requireCustomer(http.HandlerFunc(userHandler.DashboardHandler)))
	mux.Handle("GET /v1/user/{username}/transactions", requireCustomer(http.HandlerFunc(userHandler.TransactionsJSONHandler)))

	// Serve the basic frontend
	mux.Handle("/", http.FileServer(http.Dir("./frontend")))

	// Start Server
	fmt.Println("Server started on: http://" + serverAddress + ":" + port)
	if err := http.ListenAndServe(serverAddress+":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := map[string]bool{
		os.Getenv("CORS_ALLOWED_ORIGINS"): true, // Load from .env
		//"http://localhost:5173":           true, // Vue dev
		//"http://localhost:3001":           true, // Go dev
		//"https://unicard.app":   		   true, // production
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
