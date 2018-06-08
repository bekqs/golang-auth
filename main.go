package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/kataras/go-sessions"
)

var db *sql.DB
var err error

type user struct {
	ID       int
	Email    string
	Password string
}

// Connect to Database
func connectDB() {
	db, err = sql.Open("mysql", "root:rootroot@/go_db")

	if err != nil {
		log.Fatalln(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatalln(err)
	}
}

func routes() {
	http.HandleFunc("/", home)
	http.HandleFunc("/register", register)
	http.HandleFunc("/profile", profile)
	http.HandleFunc("/login", login)
	http.HandleFunc("/logout", logout)
}

func main() {
	connectDB()
	routes()

	defer db.Close()

	fmt.Println("Server running on port :8000")
	http.ListenAndServe(":8000", nil)

	err := http.ListenAndServe(":"+os.Getenv("PORT"), nil)
	if err != nil {
		panic(err)
	}
}

func checkErr(w http.ResponseWriter, r *http.Request, err error) bool {
	if err != nil {

		fmt.Println(r.Host + r.URL.Path)

		http.Redirect(w, r, r.Host+r.URL.Path, 301)
		return false
	}
	return true
}

// QueryUser - Data for user registration
func QueryUser(email string) user {
	var users = user{}
	err = db.QueryRow(`
		SELECT id, 
		email, 
		password 
		FROM users WHERE email=?
		`, email).
		Scan(
			&users.ID,
			&users.Email,
			&users.Password,
		)
	return users
}

// ProfileData - User profile details
type ProfileData struct {
	Email        string
	FullName     string
	ContactEmail string
	Address      string
	Phone        string
}

func home(w http.ResponseWriter, r *http.Request) {
	// Start session if user is logged in
	session := sessions.Start(w, r)
	sessionEmail := session.GetString("email")
	// If there is no email stored in session redirect to login page
	if len(sessionEmail) == 0 {
		http.Redirect(w, r, "/login", 301)
	}

	// Get user profile details from database
	var p ProfileData
	err = db.QueryRow("SELECT email, full_name, contact_email, address, phone FROM users where email = ?", sessionEmail).Scan(&p.Email, &p.FullName, &p.ContactEmail, &p.Address, &p.Phone)

	if err != nil {
		fmt.Println(err.Error())
	}

	fmt.Printf("Email: %s\n Full Name: %s\n Contact Email: %s\n Address: %s\n Phone: %s\n", p.Email, p.FullName, p.ContactEmail, p.Address, p.Phone)

	// Parse HTML template
	var t, err = template.ParseFiles("views/home.html")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// Data for HTML
	var data = map[string]string{
		"email":        session.GetString("email"),
		"fullName":     p.FullName,
		"contactEmail": p.ContactEmail,
		"address":      p.Address,
		"phone":        p.Phone,
	}

	// Execute HTML template with data
	t.Execute(w, data)
	return
}

func register(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.ServeFile(w, r, "views/register.html")
		return
	}

	// Get values from the form
	email := r.FormValue("email")
	password := r.FormValue("password")

	users := QueryUser(email)

	if (user{}) == users {
		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

		// If there is no error save user details to database
		if len(hashedPassword) != 0 && checkErr(w, r, err) {
			stmt, err := db.Prepare("INSERT INTO users SET email=?, password=?")
			if err == nil {
				_, err := stmt.Exec(&email, &hashedPassword)

				// Start session after sign up
				session := sessions.Start(w, r)
				session.Set("email", email)

				// Redirect to Edit profile page
				http.Redirect(w, r, "/profile", 302)

				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
	} else {
		http.Redirect(w, r, "/register", 302)
	}
}

func profile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.ServeFile(w, r, "views/profile.html")
		return
	}

	session := sessions.Start(w, r)
	sessionEmail := session.GetString("email")

	// Get values from the form
	fullName := r.FormValue("full_name")
	contactEmail := r.FormValue("contact_email")
	address := r.FormValue("address")
	phone := r.FormValue("phone")

	stmt, err := db.Prepare("UPDATE users SET full_name=?, contact_email=?, address=?, phone=? WHERE email=?")
	if err == nil {
		_, err := stmt.Exec(&fullName, &contactEmail, &address, &phone, &sessionEmail)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Redirect to home page
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
}

func login(w http.ResponseWriter, r *http.Request) {
	session := sessions.Start(w, r)

	if len(session.GetString("email")) != 0 && checkErr(w, r, err) {
		http.Redirect(w, r, "/", 302)
	}

	if r.Method != "POST" {
		http.ServeFile(w, r, "views/login.html")
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	users := QueryUser(email)

	// Check if hashed password matches to its plaintext equivalent
	var hashedPassword = bcrypt.CompareHashAndPassword([]byte(users.Password), []byte(password))

	if hashedPassword == nil {
		// Login success
		session := sessions.Start(w, r)
		session.Set("email", users.Email)
		http.Redirect(w, r, "/", 302)
	} else {
		// Login failed
		http.Redirect(w, r, "/login", 302)
	}
}

func logout(w http.ResponseWriter, r *http.Request) {
	// Destroy session and redirect to home page
	session := sessions.Start(w, r)
	session.Clear()
	sessions.Destroy(w, r)
	http.Redirect(w, r, "/", 302)
}
