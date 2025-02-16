package crypto

import (
	"bytes"
	"crypto/ed25519"
	"encoding/gob"
)

// Encoding is used to serialise the data to store the KeyPairs in the database
func init() {
	gob.Register(Ed25519PublicKey{})
	gob.Register(ed25519.PublicKey{})
	gob.Register(encKeyPair{})
}

// The KeyPair interface exposes functions relating to public and encrypted private key pairs
type KeyPair interface {
	// Accessors
	GetPublicKey() PublicKey
	GetPrivArmour() string

	// Public Key Address
	GetAddressBytes() []byte
	GetAddressString() string // hex string

	// Unarmour
	Unarmour(passphrase string) (PrivateKey, error)

	// Export
	ExportString(passphrase string) (string, error)
	ExportJSON(passphrase string) (string, error)

	// Marshalling
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
}

var _ KeyPair = &encKeyPair{}

// encKeyPair struct stores the public key and the passphrase encrypted private key
type encKeyPair struct {
	PublicKey     PublicKey
	PrivKeyArmour string
}

// Generate a new KeyPair struct given the public key and armoured private key
func newKeyPair(pub PublicKey, priv string) KeyPair {
	return &encKeyPair{
		PublicKey:     pub,
		PrivKeyArmour: priv,
	}
}

// Return an empty KeyPair interface
func GetKeypair() KeyPair {
	return &encKeyPair{}
}

// Return the public key
func (kp encKeyPair) GetPublicKey() PublicKey {
	return kp.PublicKey
}

// Return private key armoured string
func (kp encKeyPair) GetPrivArmour() string {
	return kp.PrivKeyArmour
}

// Return the byte slice address of the public key
func (kp encKeyPair) GetAddressBytes() []byte {
	return kp.PublicKey.Address().Bytes()
}

// Return the string address of the public key
func (kp encKeyPair) GetAddressString() string {
	return kp.PublicKey.Address().String()
}

// Unarmour the private key with the passphrase provided
func (kp encKeyPair) Unarmour(passphrase string) (PrivateKey, error) {
	return unarmourDecryptPrivKey(kp.PrivKeyArmour, passphrase)
}

// Export Private Key String
func (kp encKeyPair) ExportString(passphrase string) (string, error) {
	privKey, err := unarmourDecryptPrivKey(kp.PrivKeyArmour, passphrase)
	if err != nil {
		return "", err
	}
	return privKey.String(), nil
}

// Export Private Key as armoured JSON string with fields to decrypt
func (kp encKeyPair) ExportJSON(passphrase string) (string, error) {
	_, err := unarmourDecryptPrivKey(kp.PrivKeyArmour, passphrase)
	if err != nil {
		return "", err
	}
	return kp.PrivKeyArmour, nil
}

// Marshal KeyPair into a []byte
func (kp encKeyPair) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(kp); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal []byte into an encKeyPair struct
func (kp *encKeyPair) Unmarshal(bz []byte) error {
	var keyPair encKeyPair
	keypairBz := new(bytes.Buffer)
	keypairBz.Write(bz)
	dec := gob.NewDecoder(keypairBz)
	if err := dec.Decode(&keyPair); err != nil {
		return err
	}
	*kp = keyPair
	return nil
}

// Generate new private ED25519 key and encrypt and armour it as a string
// Returns a KeyPair struct of the Public Key and Armoured String
func CreateNewKey(passphrase, hint string) (KeyPair, error) {
	privKey, err := GeneratePrivateKey()
	if err != nil {
		return nil, err
	}

	privArmour, err := encryptArmourPrivKey(privKey, passphrase, hint)
	if err != nil || privArmour == "" {
		return nil, err
	}

	pubKey := privKey.PublicKey()
	kp := newKeyPair(pubKey, privArmour)

	return kp, nil
}

// Generate new KeyPair from the hex string provided, encrypt and armour it as a string
func CreateNewKeyFromString(privStrHex, passphrase, hint string) (KeyPair, error) {
	privKey, err := NewPrivateKey(privStrHex)
	if err != nil {
		return nil, err
	}

	privArmour, err := encryptArmourPrivKey(privKey, passphrase, hint)
	if err != nil || privArmour == "" {
		return nil, err
	}

	pubKey := privKey.PublicKey()
	kp := newKeyPair(pubKey, privArmour)

	return kp, nil
}

// Create new KeyPair from the JSON encoded privStr
func ImportKeyFromJSON(jsonStr, passphrase string) (KeyPair, error) {
	// Get Private Key from armouredStr
	privKey, err := unarmourDecryptPrivKey(jsonStr, passphrase)
	if err != nil {
		return nil, err
	}
	pubKey := privKey.PublicKey()
	kp := newKeyPair(pubKey, jsonStr)

	return kp, nil
}
