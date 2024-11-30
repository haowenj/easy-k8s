package comm

const (
	NodeLabelRole       = "kubernetes.io/role"
	LabelNodeRolePrefix = "node-role.kubernetes.io/"
	LabelCustomPrefix   = "osgalaxy.io"
)

var DecodeLables = map[string]struct{}{
	"osgalaxy.io/city":     {},
	"osgalaxy.io/country":  {},
	"osgalaxy.io/province": {},
}