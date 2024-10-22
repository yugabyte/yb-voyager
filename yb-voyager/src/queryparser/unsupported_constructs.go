package queryparser

import (
	"fmt"

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
	fcd := &FuncCallDetector{
		unsupportedFuncs: make(map[string]string),
	}

	for _, fname := range unsupportedAdvLockFuncs {
		fcd.unsupportedFuncs[fname] = ADVISORY_LOCKS
	}
	for _, fname := range xmlFunctions {
		fcd.unsupportedFuncs[fname] = XML_FUNCTIONS
	}
	log.Debug("initialized FuncCallDetector with unsupported functions")
	return fcd
}

// Detect checks if a FuncCall node uses an unsupported function.
func (d *FuncCallDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if getMsgFullName(msg) != PG_QUERY_FUNCCALL_NODE {
		return nil, nil
	}

	funcName := getFuncNameFromFuncCall(msg)
	log.Debugf("fetched function name from %s node: %q", PG_QUERY_FUNCCALL_NODE, funcName)
	if constructType, ok := d.unsupportedFuncs[funcName]; ok {
		log.Debugf("detected unsupported function: %s", funcName)
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
	log.Debug("initialized ColumnRefDetector with unsupported columns")
	return crd
}

// Detect checks if a ColumnRef node uses an unsupported system column
func (d *ColumnRefDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if getMsgFullName(msg) != PG_QUERY_COLUMNREF_NODE {
		return nil, nil
	}

	colName := getColNameFromColumnRef(msg)
	log.Debugf("fetched column name from %s node: %q", PG_QUERY_COLUMNREF_NODE, colName)
	if constructType, ok := d.unsupportedColumns[colName]; ok {
		log.Debugf("detected unsupported system column: %s", colName)
		return []string{constructType}, nil
	}
	return nil, nil
}

type XmlExprDetector struct{}

func NewXmlExprDetector() *XmlExprDetector {
	log.Debug("initialized XmlExprDetector")
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

// CompositeDetector combines multiple detectors into one.
type CompositeDetector struct {
	detectors []UnsupportedConstructDetector
}

// Detect applies all detectors to the given node and aggregates the results.
func (cd *CompositeDetector) Detect(msg protoreflect.Message) ([]string, error) {
	var result []string
	for _, detector := range cd.detectors {
		log.Debugf("running detector %T", detector)
		constructs, err := detector.Detect(msg)
		if err != nil {
			log.Debugf("error in detector %T: %v", detector, err)
			return nil, fmt.Errorf("error in detectors %T: %w", detector, err)
		}
		result = append(result, constructs...)
	}
	return result, nil
}
