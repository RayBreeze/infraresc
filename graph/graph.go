package graph

type Graph struct {
	Nodes map[string]*Node
}

type Node struct {
	ID           string
	ResourceType string
	Dependencies []string
}

func New() *Graph {
	return &Graph{
		Nodes: make(map[string]*Node),
	}
}

func (g *Graph) AddNode(node *Node) {
	g.Nodes[node.ID] = node
}
