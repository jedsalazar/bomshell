package functions

import (
	"os"

	"github.com/bom-squad/protobom/pkg/sbom"
	"github.com/chainguard-dev/bomshell/pkg/elements"
	"github.com/chainguard-dev/bomshell/pkg/loader"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/release-utils/version"
)

// NodeToNodeList takes a node and returns a new NodeList
// with that nodelist with the node as the only member.
var NodeToNodeList = func(lhs ref.Val) ref.Val {
	var node elements.Node
	var ok bool
	if node, ok = lhs.(elements.Node); !ok {
		return types.NewErr("attemtp to convert a non node")
	}
	return node.ToNodeList()
}

var Addition = func(lhs, rhs ref.Val) ref.Val {
	return elements.NodeList{
		NodeList: &sbom.NodeList{},
	}
}

var AdditionOp = func(vals ...ref.Val) ref.Val {
	return elements.NodeList{
		NodeList: &sbom.NodeList{},
	}
}

// ElementById returns a Node matching with the specified ID
var NodeById = func(lhs, rawID ref.Val) ref.Val {
	queryID, ok := rawID.Value().(string)
	if !ok {
		return types.NewErr("argument to element by id has to be a string")
	}
	var node *sbom.Node
	switch v := lhs.Value().(type) {
	case *sbom.Document:
		node = v.NodeList.GetNodeByID(queryID)
	case *sbom.NodeList:
		node = v.GetNodeByID(queryID)
	default:
		return types.NewErr("method unsupported on type %T", lhs.Value())
	}

	if node == nil {
		return nil
	}

	return elements.Node{
		Node: node,
	}
}

var Files = func(lhs ref.Val) ref.Val {
	nl := elements.NodeList{
		NodeList: &sbom.NodeList{},
	}
	bom, ok := lhs.Value().(*sbom.Document)
	if !ok {
		return types.NewErr("unable to convert sbom to native (wrong type?)")
	}
	nl.Edges = bom.NodeList.Edges
	for _, n := range bom.NodeList.Nodes {
		if n.Type == sbom.Node_FILE {
			nl.NodeList.Nodes = append(nl.NodeList.Nodes, n)
		}
	}
	cleanEdges(&nl)
	return nl
}

var Packages = func(lhs ref.Val) ref.Val {
	nl := elements.NodeList{
		NodeList: &sbom.NodeList{},
	}
	bom, ok := lhs.Value().(*sbom.Document)
	if !ok {
		return types.NewErr("unable to convert sbom to native (wrong type?)")
	}
	nl.Edges = bom.NodeList.Edges
	for _, n := range bom.NodeList.Nodes {
		if n.Type == sbom.Node_PACKAGE {
			nl.NodeList.Nodes = append(nl.NodeList.Nodes, n)
		}
	}
	cleanEdges(&nl)
	return nl
}

var ToDocument = func(lhs ref.Val) ref.Val {
	if lhs.Type() != elements.NodeListTypeValue {
		return types.NewErr("documents can be created only from nodelists")
	}

	nodelist, ok := lhs.(elements.NodeList)
	if !ok {
		return types.NewErr("could not cast nodelist")
	}

	// Here we reconnect all orphaned nodelists to the root of the
	// nodelist. The produced document will describe all elements of
	// the nodelist except for those which are already related to other
	// nodes in the graph.
	reconnectOrphanNodes(&nodelist)

	doc := elements.Document{
		Document: &sbom.Document{
			Metadata: &sbom.Metadata{
				Id:      "",
				Version: "1",
				Name:    "bomshell generated document",
				Date:    timestamppb.Now(),
				Tools: []*sbom.Tool{
					{
						Name:    "bomshell",
						Version: version.GetVersionInfo().GitVersion,
						Vendor:  "Chainguard Labs",
					},
				},
				Authors: []*sbom.Person{},
				Comment: "This document was generated by bomshell from a protobom nodelist",
			},
			NodeList: nodelist.NodeList,
		},
	}

	return doc
}

var LoadSBOM = func(pathVal ref.Val) ref.Val {
	path, ok := pathVal.Value().(string)
	if !ok {
		return types.NewErr("argument to element by id has to be a string")
	}

	f, err := os.Open(path)
	if err != nil {
		return types.NewErr("opening SBOM file: %w", err)
	}

	doc, err := loader.ReadSBOM(f)
	if err != nil {
		return types.NewErr("loading document: %w", err)
	}
	return elements.Document{
		Document: doc,
	}
}

var NodesByPurlType = func(lhs, rhs ref.Val) ref.Val {
	purlType, ok := rhs.Value().(string)
	if !ok {
		return types.NewErr("argument to GetNodesByPurlType must be a string")
	}

	var nl *sbom.NodeList
	switch v := lhs.Value().(type) {
	case *sbom.Document:
		nl = v.NodeList.GetNodesByPurlType(purlType)
	case *sbom.NodeList:
		nl = v.GetNodesByPurlType(purlType)
	default:
		return types.NewErr("method unsupported on type %T", lhs.Value())
	}

	return elements.NodeList{
		NodeList: nl,
	}
}

// "", sbom[2].GetNodesByPurlType("golang"), "DEPENDS_ON"
// var RelateNodeListAtID = func(lhs, rawNl, rawId, rawRel ref.Val) ref.Val {
var RelateNodeListAtID = func(vals ...ref.Val) ref.Val {
	if len(vals) != 4 {
		return types.NewErr("invalid number of arguments for RealteAtNodeListAtID")
	}
	id, ok := vals[2].Value().(string)
	if !ok {
		return types.NewErr("node id has to be a string")
	}
	// relType
	_, ok = vals[3].Value().(string)
	if !ok {
		return types.NewErr("relationship type has has to be a string")
	}

	nodelist, ok := vals[1].(elements.NodeList)
	if !ok {
		return types.NewErr("could not cast nodelist")
	}

	switch v := vals[0].Value().(type) {
	case *sbom.Document:
		// FIXME: Lookup reltype
		if err := v.NodeList.RelateNodeListAtID(nodelist.Value().(*sbom.NodeList), id, sbom.Edge_dependsOn); err != nil {
			return types.NewErr(err.Error())
		}
		return elements.Document{
			Document: v,
		}
	case *sbom.NodeList:
		if err := v.RelateNodeListAtID(nodelist.Value().(*sbom.NodeList), id, sbom.Edge_dependsOn); err != nil {
			return types.NewErr(err.Error())
		}
		return elements.NodeList{
			NodeList: v,
		}
	default:
		return types.NewErr("method unsupported on type %T", vals[0].Value())
	}
}
