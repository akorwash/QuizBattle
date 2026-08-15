package service

import "errors"

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountExists      = errors.New("account details already in use")
	ErrForbidden          = errors.New("forbidden")
	ErrAlreadyJoined      = errors.New("user already joined the battle")
	ErrGameClosed         = errors.New("battle is closed")
	ErrActiveGameLimit    = errors.New("لديك بالفعل 3 ساحات جارية؛ الساحات المنتهية لا تُحتسب ضمن الحد")
	ErrBattleFull         = errors.New("battle participant limit reached")
	ErrArenaNotReady      = errors.New("الساحة لا تستوفي عدد اللاعبين المطلوب لهذا النمط")
	ErrMatchInProgress    = errors.New("match is in progress")
)
