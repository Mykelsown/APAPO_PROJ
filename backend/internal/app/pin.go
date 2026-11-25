package app

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
)

const pinIterations = 120000

func hashPIN(pin string, salt []byte) []byte {
	k, err := pbkdf2.Key(sha256.New, pin, salt, pinIterations, 32)
	if err != nil {
		panic(err)
	}
	return k
}

func newPINHash(pin string) (hash, salt []byte) {
	salt = make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	return hashPIN(pin, salt), salt
}

func pinMatches(u *User, pin string) bool {
	if len(u.PINHash) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare(hashPIN(pin, u.PINSalt), u.PINHash) == 1
}

var weakPINs = map[string]bool{
	"0000": true, "1111": true, "2222": true, "3333": true, "4444": true,
	"5555": true, "6666": true, "7777": true, "8888": true, "9999": true,
	"1234": true, "4321": true, "0123": true, "3210": true,
}
