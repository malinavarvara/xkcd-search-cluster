package core

import "errors"

var (
	// ErrInternal — когда что-то пошло не так на стороне сервера или API
	ErrInternal = errors.New("internal error")

	// ErrNotFound — когда поиск не дал результатов
	ErrNotFound = errors.New("comics not found")

	// ErrInvalidInput — если пользователь ввел пустой запрос или некорректные данные
	ErrInvalidInput = errors.New("invalid input")

	// ErrUnauthorized — для будущей логики админки и логина[cite: 3]
	ErrUnauthorized = errors.New("unauthorized")
)
