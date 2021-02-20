package did

/*
{
  "jti": "urn:uuid:c7e23348-6d92-42ad-bf65-eac1a6767b0c",
  "iss": "did:thread:bcysaqaisecatfe3ceo2dux3hej44qr3mxoo25hkvek5ghohu7oeqe55dtdow6",
  "sub": "did:thread:bcysaqaisecatfe3ceo2dux3hej44qr3mxoo25hkvek5ghohu7oeqe55dtdow6",
  "aud": "/thread/0.0.1",
  "exp": 1613104764,
  "iat": 1613104734,
  "nbf": 1613104734,
  "vc": {
    "@context": [
      "https://www.w3.org/2018/credentials/v1"
    ],
    "type": [
      "VerifiableCredential"
    ],
    "credentialSubject": {
      "id": "did:thread:bcysaqaisecatfe3ceo2dux3hej44qr3mxoo25hkvek5ghohu7oeqe55dtdow6",
      "document": {
        "@context": [
          "https://www.w3.org/ns/did/v1"
        ],
        "id": "did:thread:bcysaqaisecatfe3ceo2dux3hej44qr3mxoo25hkvek5ghohu7oeqe55dtdow6",
        "authentication": [
          {
            "id": "did:thread:bcysaqaisecatfe3ceo2dux3hej44qr3mxoo25hkvek5ghohu7oeqe55dtdow6#keys-1",
            "type": "Ed25519VerificationKey2018",
            "controller": "did:thread:bcysaqaisecatfe3ceo2dux3hej44qr3mxoo25hkvek5ghohu7oeqe55dtdow6",
            "publicKeyBase32": "bbaareiebgkjwei5uhjpwoitzzbdwzo45v2ovkiv2mo4pj64jaj32hgg5n4"
          }
        ]
      }
    }
  }
}
*/

// DID is a concrete type for a decentralized identifier.
// See https://www.w3.org/TR/did-core/#dfn-did-schemes.
type DID string

// Document is a DID document that describes a DID subject.
// See https://www.w3.org/TR/did-core/#dfn-did-documents.
type Document struct {
	Context        []string             `json:"@context"`
	ID             string               `json:"id"`
	Conroller      []string             `json:"conroller,omitempty"`
	Authentication []VerificationMethod `json:"authentication"`
}

// VerificationMethod describes how to authenticate or authorize interactions with a DID subject.
// See https://www.w3.org/TR/did-core/#dfn-verification-method.
type VerificationMethod struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Controller         string `json:"controller"`
	PublicKeyMultiBase string `json:"publicKeyMultiBase"`
}

// VerifiableCredential is a set of claims about a subject.
// See https://www.w3.org/TR/vc-data-model/#dfn-verifiable-credentials.
type VerifiableCredential struct {
	Context           []string                    `json:"@context"`
	Type              []string                    `json:"type"`
	CredentialSubject VerifiableCredentialSubject `json:"credentialSubject"`
}

// VerifiableCredentialSubject is an entity about which claims are made.
// See https://www.w3.org/TR/vc-data-model/#dfn-subjects.
type VerifiableCredentialSubject struct {
	ID       string   `json:"id"`
	Document Document `json:"document,omitempty"`
}

// Token is a concrete type for a JWT token string.
type Token string

// Defined returns true if token is not empty.
func (t Token) Defined() bool {
	return t != ""
}
