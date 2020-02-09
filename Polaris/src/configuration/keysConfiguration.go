package configuration

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"strings"
)

type KeysConfig struct {
	id               string
	serverPublicKeys map[string]*rsa.PublicKey
	clientPublicKeys map[string]*rsa.PublicKey
	privateKey       *rsa.PrivateKey
}

func NewKeysConfig(configHome string, keyPath string, id string) *KeysConfig {
	k := &KeysConfig{
		id:               id,
		serverPublicKeys: make(map[string]*rsa.PublicKey),
		clientPublicKeys: make(map[string]*rsa.PublicKey),
	}
	k.loadKeys(configHome, keyPath)
	return k
}

func LoadPublicKey(fileName string) *rsa.PublicKey {
	pub, err := ioutil.ReadFile(fileName)

	var parsedKey interface{}
	var ok bool
	var pubKey *rsa.PublicKey

	if err != nil {
		logger.Fatalf("Cannot open the public key file for file: %v", fileName)
	}

	pubPem, _ := pem.Decode(pub)
	if pubPem == nil {
		logger.Fatal("cannot decode the public key for server/client", fileName)
	}

	if pubPem.Type != "RSA PUBLIC KEY" {
		logger.Fatal("the type of public key for server/client", fileName)
	}

	if parsedKey, err = x509.ParsePKCS1PublicKey(pubPem.Bytes); err == nil {
		if pubKey, ok = parsedKey.(*rsa.PublicKey); ok {
			return pubKey
		} else {
			logger.Fatal("cannot cast to rsa public key for server/client", fileName)
		}
	} else {
		logger.Fatal("cannot parse the public key for server/client", fileName)
	}
	return nil
}

func LoadPrivateKey(file string) *rsa.PrivateKey {
	priv, err := ioutil.ReadFile(file)
	if err != nil {
		logger.Fatal("cannot read the private key file for server", file)
	}

	privPem, _ := pem.Decode(priv)
	if privPem == nil {
		logger.Fatal("cannot decode the private key for server", file)
	}

	if privPem.Type != "RSA PRIVATE KEY" {
		logger.Fatal("cannot decode the private key for server", file)
	}

	var parsedKey interface{}
	var ok bool
	var privKey *rsa.PrivateKey
	if parsedKey, err = x509.ParsePKCS1PrivateKey(privPem.Bytes); err == nil {
		if privKey, ok = parsedKey.(*rsa.PrivateKey); ok {
			return privKey
		} else {
			logger.Fatal("cannot cast to rsa private key for server", file)
		}
	} else {
		logger.Fatal("cannot parse the private key for server", file)
	}

	return nil
}

func LoadPublicKeys(keyDir string) map[string]*rsa.PublicKey {
	files, err := ioutil.ReadDir(keyDir)
	if err != nil {
		logger.Fatal(err)
	}

	publicKeyMap := make(map[string]*rsa.PublicKey)

	for _, f := range files {
		items := strings.Split(f.Name(), ".")
		id := items[1]

		logger.Info("load public key for client/server", id)
		pubKey := LoadPublicKey(keyDir + "/" + f.Name())
		if pubKey != nil {
			publicKeyMap[id] = pubKey
		} else {
			logger.Fatalf("cannot load public key for client/server %v", id)
		}
	}

	return publicKeyMap
}

func (k *KeysConfig) loadKeys(configHome string, keyPath string) {

	path := ""

	if configHome == "" && keyPath == "" {
		path = "config/keys"
	} else if configHome == "" {
		path = "config/" + keyPath
	} else if keyPath == "" {
		path = configHome + "/keys"
	} else {
		path = configHome + "/" + keyPath
	}

	//	var parsedKey interface{}
	//	var ok bool
	//var pubKey *rsa.PublicKey
	// load the public keys for servers
	serverKeyPath := path + "/server_public"
	k.serverPublicKeys = LoadPublicKeys(serverKeyPath)

	//load the public keys for client
	clientKeyPath := path + "/client_public"
	k.clientPublicKeys = LoadPublicKeys(clientKeyPath)

	// load the private key for this server for signature
	logger.Infof("load Private key for server %v", k.id)
	file := fmt.Sprintf("%v/server_private/server.%v.priv.pem", path, k.id)
	k.privateKey = LoadPrivateKey(file)

}

func (k *KeysConfig) GetPrivateKey() *rsa.PrivateKey {
	return k.privateKey
}

func (k *KeysConfig) GetPublicKeyById(id string) *rsa.PublicKey {
	if key, ok := k.serverPublicKeys[id]; ok {
		return key
	}

	return nil
}

func (k *KeysConfig) GetClientPublicKeyById(id string) *rsa.PublicKey {
	if key, ok := k.clientPublicKeys[id]; ok {
		return key
	}
	return nil
}
