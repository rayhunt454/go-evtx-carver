package binxml

// Байты токена потока BinXML (MS-EVEN6 §2.2.3.1).
const (
	tokEOF                     = 0x00
	tokOpenStartElement        = 0x01
	tokCloseStartElement       = 0x02
	tokCloseEmptyElement       = 0x03
	tokCloseElement            = 0x04
	tokValue                   = 0x05
	tokValueVariant            = 0x45
	tokAttribute               = 0x06
	tokAttributeVariant        = 0x46
	tokCDataSection            = 0x07
	tokCDataSectionVariant     = 0x47
	tokCharRef                 = 0x08
	tokCharRefVariant          = 0x48
	tokEntityRef               = 0x09
	tokEntityRefVariant        = 0x49
	tokPITarget                = 0x0a
	tokPIData                  = 0x0b
	tokTemplateInstance        = 0x0c
	tokSubstitution            = 0x0d
	tokSubstitutionOptional    = 0x0e
	tokFragmentHeader          = 0x0f
	tokOpenStartElementHasAttr = 0x41
)
