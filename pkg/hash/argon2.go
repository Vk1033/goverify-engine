package hash

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type Config struct {
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
}

var DefaultConfig = Config{
	Time:    1,
	Memory:  64 * 1024,
	Threads: 4,
	KeyLen:  32,
}

// GenerateDemographicHash takes DOB and Gender, generates a salt, and returns a secure hash string.
func GenerateDemographicHash(dob, gender string, cfg Config) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	data := []byte(fmt.Sprintf("%s|%s", dob, strings.ToLower(gender)))
	hash := argon2.IDKey(data, salt, cfg.Time, cfg.Memory, cfg.Threads, cfg.KeyLen)

	// Format: $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, cfg.Memory, cfg.Time, cfg.Threads, b64Salt, b64Hash)

	return encoded, nil
}

// CompareDemographicHash verifies if the given dob and gender match the stored encoded hash.
func CompareDemographicHash(dob, gender, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, err
	}
	if version != argon2.Version {
		return false, fmt.Errorf("incompatible version")
	}

	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	keyLen := uint32(len(decodedHash))

	data := []byte(fmt.Sprintf("%s|%s", dob, strings.ToLower(gender)))
	hash := argon2.IDKey(data, salt, time, memory, threads, keyLen)

	if len(hash) != len(decodedHash) {
		return false, nil
	}
	for i := range hash {
		if hash[i] != decodedHash[i] {
			return false, nil
		}
	}
	return true, nil
}
