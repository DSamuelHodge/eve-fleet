package cli

import (
	"errors"

	"github.com/DSamuelHodge/eve-fleet/internal/diag"
)

var errPrinted = errors.New("printed")

func (g *globals) report(r diag.Report) error {
	w := g.stdout
	if !g.JSON && !r.OK {
		w = g.stderr
	}
	var err error
	if g.JSON {
		err = r.WriteJSON(w)
	} else {
		err = r.WritePlain(w)
	}
	if err != nil {
		return err
	}
	if !r.OK {
		return errPrinted
	}
	return nil
}
