package uni

import (
	"encoding/json"
	"io"
)

func ExampleContext_LoadModule() {
	// this whole first part is just setting up for the example;
	// note the struct tags - very important; we specify inline_key
	// because that is the only way to know the module name
	var ctx Context
	myStruct := &struct {
		// This godoc comment will appear in module documentation.
		GuestModuleRaw json.RawMessage `json:"guest_module,omitempty" caddy:"namespace=example inline_key=name"`

		// this is where the decoded module will be stored; in this
		// example, we pretend we need an io.Writer but it can be
		// any interface type that is useful to you
		guestModule io.Writer
	}{
		GuestModuleRaw: json.RawMessage(`{"name":"module_name","foo":"bar"}`),
	}

	// if a guest module is provided, we can load it easily
	if myStruct.GuestModuleRaw != nil {
		mod, err := ctx.LoadModule(myStruct, "GuestModuleRaw")
		if err != nil {
			// you'd want to actually handle the error here
			// return fmt.Errorf("loading guest module: %v", err)
		}
		// mod contains the loaded and provisioned module,
		// it is now ready for us to use
		myStruct.guestModule = mod.(io.Writer)
	}

	// use myStruct.guestModule from now on

}
