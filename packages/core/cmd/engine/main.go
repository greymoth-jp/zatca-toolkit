// Command engine is the deterministic invoice processing core (validation + conversion).
// It is invoked by apps/api over stdin/stdout JSON (a gRPC server is a later ticket).
// Contract: read one JSON Request from stdin, write one JSON Response to stdout.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/greymoth-jp/zatca-toolkit/core/internal/convert"
	"github.com/greymoth-jp/zatca-toolkit/core/internal/normalized"
	"github.com/greymoth-jp/zatca-toolkit/core/internal/validate"
)

// Request is the engine command envelope.
type Request struct {
	Op      string           `json:"op"`      // "validate" | "convert"
	Profile string           `json:"profile"` // "en16931" | "peppol-bis"
	Format  string           `json:"format"`  // for convert: "ubl"
	Doc     normalized.Doc   `json:"doc"`
}

// Response is the engine result envelope.
type Response struct {
	OK     bool             `json:"ok"`
	Report *validate.Report `json:"report,omitempty"`
	Artifact string         `json:"artifact,omitempty"` // generated XML (convert)
	Error  string           `json:"error,omitempty"`
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail("read stdin: " + err.Error())
	}
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		fail("parse request: " + err.Error())
	}

	switch req.Op {
	case "validate":
		profile := validate.ProfilePeppol
		if req.Profile == string(validate.ProfileEN16931) {
			profile = validate.ProfileEN16931
		}
		rep := validate.Validate(&req.Doc, profile)
		emit(Response{OK: rep.Valid, Report: &rep})
	case "convert":
		var out []byte
		var err error
		switch req.Format {
		case "", "ubl":
			out, err = convert.ToUBL(&req.Doc)
		case "cii":
			out, err = convert.ToCII(&req.Doc)
		case "facturx":
			pkg, e := convert.BuildFacturX(&req.Doc, convert.ProfileEN16931FX)
			if e != nil {
				fail("convert: " + e.Error())
			}
			// emit the embedded CII payload; XMP/PDF-A3 assembly is the writer's job.
			out = pkg.Payload
		default:
			fail("unknown format: " + req.Format)
		}
		if err != nil {
			fail("convert: " + err.Error())
		}
		emit(Response{OK: true, Artifact: string(out)})
	default:
		fail("unknown op: " + req.Op)
	}
}

func emit(r Response) {
	b, _ := json.Marshal(r)
	fmt.Println(string(b))
}

func fail(msg string) {
	b, _ := json.Marshal(Response{OK: false, Error: msg})
	fmt.Println(string(b))
	os.Exit(1)
}
