package queryparser

import (
	"fmt"

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
	fcd := &FuncCallDetector{
		unsupportedFuncs: make(map[string]string),
	}

	for _, fname := range unsupportedAdvLockFuncs {
		fcd.unsupportedFuncs[fname] = ADVISORY_LOCKS
	}
	for _, fname := range xmlFunctions {
		fcd.unsupportedFuncs[fname] = XML_FUNCTIONS
	}
	return fcd
}

// Detect checks if a FuncCall node uses an unsupported function.
func (d *FuncCallDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if getMsgFullName(msg) != PG_QUERY_FUNCCALL_NODE {
		return nil, nil
	}

	funcName := getFuncNameFromFuncCall(msg)
	if constructType, ok := d.unsupportedFuncs[funcName]; ok {
		return []string{constructType}, nil
	}
	return nil, nil
}

type ColumnRefDetector struct {
	unsupportedColumns map[string]string
}

func NewColumnRefDetector() *ColumnRefDetector {
	crd := &ColumnRefDetector{
		unsupportedColumns: make(map[string]string),
	}

	for _, colName := range unsupportedSysCols {
		crd.unsupportedColumns[colName] = SYSTEM_COLUMNS
	}
	return crd
}

// Detect checks if a ColumnRef node uses an unsupported system column
func (d *ColumnRefDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if getMsgFullName(msg) != PG_QUERY_COLUMNREF_NODE {
		return nil, nil
	}

	colName := getColNameFromColumnRef(msg)
	if constructType, ok := d.unsupportedColumns[colName]; ok {
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
		return []string{XML_FUNCTIONS}, nil
	}
	return nil, nil
}

// CompositeDetector combines multiple detectors into one.
type CompositeDetector struct {
	detectors []UnsupportedConstructDetector
}

// Detect applies all detectors to the given node and aggregates the results.
func (cd *CompositeDetector) Detect(msg protoreflect.Message) ([]string, error) {
	var result []string
	for _, detector := range cd.detectors {
		constructs, err := detector.Detect(msg)
		if err != nil {
			return nil, fmt.Errorf("error in detectors %T: %w", detector, err)
		}
		result = append(result, constructs...)
	}
	return result, nil
}
