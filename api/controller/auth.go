package controller

import "net/http"

//AuthController game controller
type AuthController struct{}

//SignInPage SignIn page http requst handler
func (controller *AuthController) SignInPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/auth/signin" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.ServeFile(w, r, "./api/view/signin.html")
}
