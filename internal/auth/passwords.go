package auth

import (
	"encoding/base64"
	"fmt"
	"unicode/utf8"

	"github.com/alexedwards/argon2id"
)

var passwordParams = argon2id.Params{
	Memory:      64 * 1024,
	Iterations:  3,
	Parallelism: 4,
	SaltLength:  16,
	KeyLength:   32,
}

func validatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if !utf8.ValidString(password) || length < 4 || length > 1024 {
		return fmt.Errorf("Пароль должен содержать от 4 до 1024 символов")
	}
	return nil
}

func HashPassword(password string) (string, error) {
	if err := validatePassword(password); err != nil {
		return "", err
	}
	return argon2id.CreateHash(password, &passwordParams)
}

func checkPassword(password, encoded string) (bool, *argon2id.Params) {
	if validatePassword(password) != nil || len(encoded) > 1024 {
		return false, nil
	}
	settings, salt, hash, err := argon2id.DecodeHash(encoded)
	if err != nil {
		return false, nil
	}
	// Bound resource costs before the library allocates Argon2 memory.
	if settings.Parallelism < 1 || settings.Parallelism > 16 ||
		settings.Memory < 8*uint32(settings.Parallelism) || settings.Memory > 256*1024 ||
		settings.Iterations < 1 || settings.Iterations > 10 ||
		settings.SaltLength < 8 || settings.SaltLength > 64 ||
		settings.KeyLength < 16 || settings.KeyLength > 64 {
		return false, nil
	}
	// DecodeHash also accepts trailing text and line breaks; require the exact format.
	canonical := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		settings.Memory, settings.Iterations, settings.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash))
	if encoded != canonical {
		return false, nil
	}
	matched, err := argon2id.ComparePasswordAndHash(password, encoded)
	return matched && err == nil, settings
}

func VerifyPassword(password, encoded string) bool {
	matched, _ := checkPassword(password, encoded)
	return matched
}
