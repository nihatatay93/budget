package main

// Embed the IANA timezone database so workspace validation does not depend on
// timezone files being installed in the minimal production container.
import _ "time/tzdata"
