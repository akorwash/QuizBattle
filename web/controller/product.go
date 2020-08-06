package web

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func (a *App) getProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	/*p := product{ID: id}
	    if err := p.getProduct(a.DB); err != nil {
	        switch err {
	        case sql.ErrNoRows:
	            respondWithError(w, http.StatusNotFound, "Product not found")
	        default:
	            respondWithError(w, http.StatusInternalServerError, err.Error())
	        }
	        return
		}*/

	fmt.Println(id)
	respondWithJSON(w, http.StatusOK, id)
}
