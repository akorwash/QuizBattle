package repositorytests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetQuestionByID(t *testing.T) {
	fakeQuestion, err := seedtestQuestion()
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
		assert.NoError(t, err)
	}
	retQuestion, rerr := qeustionRepo.GetQuestionByID(fakeQuestion.ID)
	if rerr != nil {
		fmt.Printf("This is the error %v\n", rerr)
		assert.NoError(t, rerr)
	}

	assert.Equal(t, fakeQuestion, retQuestion)

	retQuestion, rerr = qeustionRepo.GetQuestionByID(fakeQuestion.ID + 10)
	if rerr != nil {
		fmt.Printf("This is the error %v\n", rerr)
		assert.NoError(t, rerr)
	}

	assert.NotEqual(t, fakeQuestion, retQuestion)

	err = deletetestQuestion(fakeQuestion)
	if err != nil {
		fmt.Printf("This is the error %v\n", err)
		assert.NoError(t, err)
	}
}
