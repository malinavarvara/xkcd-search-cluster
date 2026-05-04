package core

import "errors"

const MaxPhraseSize = 16 * 1024

var (
	ErrEmptyPhrase          = errors.New("phrase is empty")
	ErrPhraseTooLarge       = errors.New("phrase exceeds 16 KiB limit")
	ErrServiceUnavailable   = errors.New("words service unavailable")
	ErrRequestTimeout       = errors.New("request timeout")
	ErrInvalidArgument      = errors.New("invalid argument")
	ErrUpdateAlreadyRunning = errors.New("update already running")
)
