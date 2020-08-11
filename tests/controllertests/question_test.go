package controllertests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestGetQuestionByID(t *testing.T) {
	questions, err := seedtestQuestions()
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}

	samples := []struct {
		ID         int
		statusCode int
	}{
		{
			ID:         10,
			statusCode: 200,
		},
		{
			ID:         20,
			statusCode: 200,
		},
		{
			ID:         30,
			statusCode: 200,
		},
		{
			ID:         40,
			statusCode: 200,
		},
		{
			ID:         50,
			statusCode: 404,
		},
	}

	for _, v := range samples {
		PATH := "/question/" + strconv.Itoa(v.ID)
		r, err := http.NewRequest("GET", PATH, nil)
		if err != nil {
			t.Errorf("this is the error: %v", err)
		}
		rr := httptest.NewRecorder()

		Router().ServeHTTP(rr, r)

		if !assert.Equal(t, rr.Code, v.statusCode) {
			fmt.Println(rr.Code, v)
		}
	}

	_, err = deleteSeedtestQuestions(questions)
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
	}
}

func Router() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/question/{id}", questionController.GetQuestionByID)

	return r
}
