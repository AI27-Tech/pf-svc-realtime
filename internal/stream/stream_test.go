package stream

import "testing"

// El round-trip codificar→parsear es la única lógica no trivial del paquete
// (un parser), así que lleva su autochequeo (golang-best-practices §7).
func TestCodificarParsearRoundTrip(t *testing.T) {
	original := Posicion{StreamID: "linea-abc", Epoch: 3, Seq: 42}
	codificado := CodificarLastEventID(original)

	vuelta, err := ParsearLastEventID(codificado)
	if err != nil {
		t.Fatalf("parsear un valor recién codificado no debe fallar: %v", err)
	}
	if vuelta != original {
		t.Fatalf("round-trip perdió información: original=%+v vuelta=%+v", original, vuelta)
	}
}

// Un Last-Event-ID malformado se reporta como error para que el handler
// responda 409 resync, no como un pánico ni una posición silenciosamente cero.
func TestParsearInvalido(t *testing.T) {
	casos := map[string]string{
		"vacío":             "",
		"sin seq":           "linea.3",
		"epoch no numérico": "linea.x.1",
		"seq no numérico":   "linea.3.y",
		"stream_id vacío":   ".3.1",
	}
	for nombre, valor := range casos {
		t.Run(nombre, func(t *testing.T) {
			if _, err := ParsearLastEventID(valor); err == nil {
				t.Fatalf("se esperaba error para %q", valor)
			}
		})
	}
}
