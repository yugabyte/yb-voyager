package queryparser

import (
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	ADVISORY_LOCKS = "Advisory Locks"
	SYSTEM_COLUMNS = "System Columns"
	XML_FUNCTIONS  = "XML Functions"
)

// To Add a new unsupported query construct implement this interface
type UnsupportedConstructDetector interface {
	Detect(msg protoreflect.Message) ([]string, error)
}

type FuncCallDetector struct {
	// right now it covers Advisory Locks and XML functions
	unsupportedFuncs map[string]string
}

func NewFuncCallDetector() *FuncCallDetector {
	unsupportedFuncs := make(map[string]string)
	for _, fname := range unsupportedAdvLockFuncs {
		unsupportedFuncs[fname] = ADVISORY_LOCKS
	}
	for _, fname := range xmlFunctions {
		unsupportedFuncs[fname] = XML_FUNCTIONS
	}

	return &FuncCallDetector{
		unsupportedFuncs: unsupportedFuncs,
	}
}

// Detect checks if a FuncCall node uses an unsupported function.
func (d *FuncCallDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if getMsgFullName(msg) != PG_QUERY_FUNCCALL_NODE {
		return nil, nil
	}

	funcName := getFuncNameFromFuncCall(msg)
	log.Debugf("fetched function name from %s node: %q", PG_QUERY_FUNCCALL_NODE, funcName)
	if constructType, isUnsupported := d.unsupportedFuncs[funcName]; isUnsupported {
		log.Debugf("detected unsupported function %q in msg - %+v", funcName, msg)
		return []string{constructType}, nil
	}
	return nil, nil
}

type ColumnRefDetector struct {
	unsupportedColumns map[string]string
}

func NewColumnRefDetector() *ColumnRefDetector {
	unsupportedColumns := make(map[string]string)
	for _, colName := range unsupportedSysCols {
		unsupportedColumns[colName] = SYSTEM_COLUMNS
	}

	return &ColumnRefDetector{
		unsupportedColumns: unsupportedColumns,
	}
}

// Detect checks if a ColumnRef node uses an unsupported system column
func (d *ColumnRefDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if getMsgFullName(msg) != PG_QUERY_COLUMNREF_NODE {
		return nil, nil
	}

	colName := getColNameFromColumnRef(msg)
	log.Debugf("fetched column name from %s node: %q", PG_QUERY_COLUMNREF_NODE, colName)
	if constructType, isUnsupported := d.unsupportedColumns[colName]; isUnsupported {
		log.Debugf("detected unsupported system column %q in msg - %+v", colName, msg)
		return []string{constructType}, nil
	}
	return nil, nil
}

type XmlExprDetector struct{}

func NewXmlExprDetector() *XmlExprDetector {
	return &XmlExprDetector{}
}

// Detect checks if a XmlExpr node is present, means Xml type/functions are used
func (d *XmlExprDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if getMsgFullName(msg) == PG_QUERY_XMLEXPR_NODE {
		log.Debug("detected xml expression")
		return []string{XML_FUNCTIONS}, nil
	}
	return nil, nil
}
