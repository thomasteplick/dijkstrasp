/*
This is a web application.  The backend server is written in Go and uses the
html/package to create the html used by the web browser, which points to localhost:8080/dijkstrasp.
Dijkstra shortest paths finds the shortest paths (SP) between a source vertex and all other vertices.
Plot the SP showing the vertices and edges connecting the chosen source and target.
The user enters the following data in an html form:  #vertices and  x-y Euclidean bounds.
A random number of vertices is chosen for the connection with a random start vertex.
The user can select the source and target vertices of the shortest path to find.  Their
coordinates are displayed as well as their distance.
*/

package main

import (
	"bufio"
	"container/heap"
	"fmt"
	"log"
	"math"
	"math/cmplx"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"
)

const (
	addr                = "127.0.0.1:8080"              // http server listen address
	fileDijkstraSP      = "templates/dijkstrasp.html"   // html for Dijkstra SP
	fileGraphOptions    = "templates/graphoptions.html" // html for Graph Options
	patternDijkstraSP   = "/dijkstrasp"                 // http handler for Dijkstra SP connections
	patternGraphOptions = "/graphoptions"               // http handler for Graph Options
	rows                = 300                           // #rows in grid
	columns             = rows                          // #columns in grid
	xlabels             = 11                            // # labels on x axis
	ylabels             = 11                            // # labels on y axis
	fileVerts           = "vertices.csv"                // bounds and complex locations of vertices
)

// Edges are the vertices of the edge endpoints
type Edge struct {
	v int // one vertix
	w int // the other vertix
}

// Items are stored in the Priority Queue
type Item struct {
	Edge             // embedded field accessed with v,w
	index    int     // The index is used by Priority Queue update and is maintained by the heap.Interface
	distance float64 // Edge distance between vertices
}

// Priority Queue is a map of indexes and queue items and implements the heap.Interface
// A map is used instead of a slice so that it can be easily determined if an edge is in the queue
type PriorityQueue map[int]*Item

// Minimum spanning tree holds the edge vertices
type MST []*Edge

// Type to contain all the HTML template actions
type PlotT struct {
	Grid           []string // plotting grid
	Status         string   // status of the plot
	Xlabel         []string // x-axis labels
	Ylabel         []string // y-axis labels
	Distance       string   // Prim MST total distance (all the edges in MST)
	Vertices       string   // number of vertices
	Xmin           string   // x minimum endpoint in Euclidean graph
	Xmax           string   // x maximum endpoint in Euclidean graph
	Ymin           string   // y minimum endpoint in Euclidean graph
	Ymax           string   // y maximum endpoint in Euclidean graph
	StartLocation  string   // Prim MST start vertex location in x,y coordinates
	SourceLocation string   // source vertex for Dijkstra SP in x,y coordinates
	TargetLocation string   // target or destination vertex for Dijkstra SP in x,y coordinates
	Source         string   // source vertex for Dijkstra SP 0-Vertices-1
	Target         string   // target vertex for Dijkstra SP 0-Vertices-1
	DistanceSP     string   // shortest path distance (source->target)
}

// Type to hold the minimum and maximum data values of the Euclidean graph
type Endpoints struct {
	xmin float64
	xmax float64
	ymin float64
	ymax float64
}

// PrimMST type for Minimum Spanning Tree methods
type PrimMST struct {
	graph      [][]float64  // matrix of vertices and their distance from each other
	location   []complex128 // complex point(x,y) coordinates of vertices
	mst        MST
	*Endpoints // Euclidean graph endpoints
	plot       *PlotT
}

// DijkstraSP type for Shortest Path methods
type DijksraSP struct {
	edgeTo     []*Edge      // edge to vertex w
	distTo     []float64    // distance to w from source
	adj        [][]*Edge    // adjacency list
	mst        MST          // reference PrimMST
	graph      [][]float64  // reference PrimMST
	location   []complex128 // reference PrimMST
	plot       *PlotT       // reference PrimMST
	source     int          // start vertex for shortest path
	target     int          // end vertex for shortest path
	*Endpoints              // Euclidean graph endpoints
}

// global variables for parse and execution of the html template and MST construction
var (
	tmplForm *template.Template
)

// init parses the html template fileS
func init() {
	tmplForm = template.Must(template.ParseFiles(fileDijkstraSP))
}

// generateVertices creates random vertices in the complex plane
func (p *PrimMST) generateVertices(r *http.Request) error {

	// if Source and Target have values, then graph was saved and
	// we are going to calculate the SP.
	sourceVert := r.PostFormValue("sourcevert")
	targetVert := r.PostFormValue("targetvert")
	if len(sourceVert) > 0 && len(targetVert) > 0 {
		f, err := os.Open(fileVerts)
		if err != nil {
			fmt.Printf("Open file %s error: %v\n", fileVerts, err)
		}
		defer f.Close()
		input := bufio.NewScanner(f)
		input.Scan()
		line := input.Text()
		// Each line has comma-separated values
		values := strings.Split(line, ",")
		var xmin, ymin, xmax, ymax float64
		if xmin, err = strconv.ParseFloat(values[0], 64); err != nil {
			fmt.Printf("String %s conversion to float error: %v\n", values[0], err)
			return err
		}

		if ymin, err = strconv.ParseFloat(values[1], 64); err != nil {
			fmt.Printf("String %s conversion to float error: %v\n", values[1], err)
			return err
		}
		if xmax, err = strconv.ParseFloat(values[2], 64); err != nil {
			fmt.Printf("String %s conversion to float error: %v\n", values[2], err)
			return err
		}

		if ymax, err = strconv.ParseFloat(values[3], 64); err != nil {
			fmt.Printf("String %s conversion to float error: %v\n", values[3], err)
			return err
		}
		p.Endpoints = &Endpoints{xmin: xmin, ymin: ymin, xmax: xmax, ymax: ymax}

		p.location = make([]complex128, 0)
		for input.Scan() {
			line := input.Text()
			// Each line has comma-separated values
			values := strings.Split(line, ",")
			var x, y float64
			if x, err = strconv.ParseFloat(values[0], 64); err != nil {
				fmt.Printf("String %s conversion to float error: %v\n", values[0], err)
				continue
			}
			if y, err = strconv.ParseFloat(values[1], 64); err != nil {
				fmt.Printf("String %s conversion to float error: %v\n", values[1], err)
				continue
			}
			p.location = append(p.location, complex(x, y))
		}

		return nil
	}
	// Generate V vertices and locations randomly, get from HTML form
	// or read in from a previous graph when using a new start vertex.
	// Insert vertex complex coordinates into locations
	str := r.FormValue("xmin")
	xmin, err := strconv.ParseFloat(str, 64)
	if err != nil {
		fmt.Printf("String %s conversion to float error: %v\n", str, err)
		return err
	}

	str = r.FormValue("ymin")
	ymin, err := strconv.ParseFloat(str, 64)
	if err != nil {
		fmt.Printf("String %s conversion to float error: %v\n", str, err)
		return err
	}

	str = r.FormValue("xmax")
	xmax, err := strconv.ParseFloat(str, 64)
	if err != nil {
		fmt.Printf("String %s conversion to float error: %v\n", str, err)
		return err
	}

	str = r.FormValue("ymax")
	ymax, err := strconv.ParseFloat(str, 64)
	if err != nil {
		fmt.Printf("String %s conversion to float error: %v\n", str, err)
		return err
	}

	// Check if xmin < xmax and ymin < ymax and correct if necessary
	if xmin >= xmax {
		xmin, xmax = xmax, xmin
	}
	if ymin >= ymax {
		ymin, ymax = ymax, ymin
	}

	p.Endpoints = &Endpoints{xmin: xmin, ymin: ymin, xmax: xmax, ymax: ymax}

	vertices := r.FormValue("vertices")
	verts, err := strconv.Atoi(vertices)
	if err != nil {
		fmt.Printf("String %s conversion to int error: %v\n", vertices, err)
		return err
	}

	delx := xmax - xmin
	dely := ymax - ymin
	// Generate vertices
	p.location = make([]complex128, verts)
	for i := 0; i < verts; i++ {
		x := xmin + delx*rand.Float64()
		y := ymin + dely*rand.Float64()
		p.location[i] = complex(x, y)
	}

	// Save the endpoints and vertex locations to a csv file
	f, err := os.Create(fileVerts)
	if err != nil {
		fmt.Printf("Create file %s error: %v\n", fileVerts, err)
		return err
	}
	defer f.Close()
	// Save the endpoints
	fmt.Fprintf(f, "%f,%f,%f,%f\n", p.xmin, p.ymin, p.xmax, p.ymax)
	// Save the vertex locations as x,y
	for _, z := range p.location {
		fmt.Fprintf(f, "%f,%f\n", real(z), imag(z))
	}

	return nil
}

// findDistances find distances between vertices and insert into graph
func (p *PrimMST) findDistances() error {

	verts := len(p.location)
	// Store distances between vertices for Euclidean graph
	p.graph = make([][]float64, verts)
	for i := 0; i < verts; i++ {
		p.graph[i] = make([]float64, verts)
	}

	for i := 0; i < verts; i++ {
		for j := i + 1; j < verts; j++ {
			distance := cmplx.Abs(p.location[i] - p.location[j])
			p.graph[i][j] = distance
			p.graph[j][i] = distance
		}
	}
	for i := 0; i < verts; i++ {
		p.graph[i][i] = math.MaxFloat64
	}

	return nil
}

// A PriorityQueue implements heap.Interface and holds Items
// Len returns length of queue.
func (pq PriorityQueue) Len() int {
	return len(pq)
}

// Less returns Item weight[i] less than Item weight[j]
func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].distance < pq[j].distance
}

// Swap swaps Item[i] and Item[j]
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], (pq)[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push inserts an Item in the queue
func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	(*pq)[n] = item
}

// Pop removes an Item from the queue and returns it
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	delete(*pq, n-1)
	return item
}

// update modifies the distance and value of an Item in the queue
func (pq *PriorityQueue) update(item *Item, distance float64) {
	item.distance = distance
	heap.Fix(pq, item.index)
}

// findMST finds the minimum spanning tree (MST) using Prim's algorithm
func (p *PrimMST) findMST() error {
	vertices := len(p.location)
	p.mst = make(MST, vertices)
	marked := make([]bool, vertices)
	distTo := make([]float64, vertices)
	for i := range distTo {
		distTo[i] = math.MaxFloat64
	}
	// Create a priority queue, put the items in it, and establish
	// the priority queue (heap) invariants.
	pq := make(PriorityQueue)

	visit := func(v int) {
		marked[v] = true
		// find shortest distance from vertex v to w
		for w, dist := range p.graph[v] {
			// Check if already in the MST
			if marked[w] {
				continue
			}
			if dist < distTo[w] {
				// Edge to w is new best connection from MST to w
				p.mst[w] = &Edge{v: v, w: w}
				distTo[w] = dist
				// Check if already in the queue and update
				item, ok := pq[w]
				// update
				if ok {
					pq.update(item, dist)
				} else { // insert
					item = &Item{Edge: Edge{v: v, w: w}, distance: dist}
					heap.Push(&pq, item)
				}
			}
		}
	}

	// Starting index is 0, distance is MaxFloat64, put it in the queue
	distTo[0] = math.MaxFloat64
	pq[0] = &Item{index: 0, distance: math.MaxFloat64, Edge: Edge{v: 0, w: 0}}
	heap.Init(&pq)

	// Loop until the queue is empty and the MST is finished
	for pq.Len() > 0 {
		item := heap.Pop(&pq).(*Item)
		visit(item.w)
	}

	return nil
}

// plotMST draws the MST onto the grid
func (p *PrimMST) plotMST(status []string) error {

	// Apply the parsed HTML template to plot object
	// Construct x-axis labels, y-axis labels, status message

	var (
		xscale   float64
		yscale   float64
		distance float64
	)
	p.plot = &PlotT{}
	p.plot.Grid = make([]string, rows*columns)
	p.plot.Xlabel = make([]string, xlabels)
	p.plot.Ylabel = make([]string, ylabels)

	// Calculate scale factors for x and y
	xscale = (columns - 1) / (p.xmax - p.xmin)
	yscale = (rows - 1) / (p.ymax - p.ymin)

	// Insert the mst vertices and edges in the grid
	// loop over the MST vertices

	// color the vertices black
	// color the edges connecting the vertices gray
	// color the MST start vertex green
	// create the line y = mx + b for each edge
	// translate complex coordinates to row/col on the grid
	// translate row/col to slice data object []string Grid
	// CSS selectors for background-color are "vertex", "startvertexMSS", and "edge"

	beginEP := complex(p.xmin, p.ymin)  // beginning of the Euclidean graph
	endEP := complex(p.xmax, p.ymax)    // end of the Euclidean graph
	lenEP := cmplx.Abs(endEP - beginEP) // length of the Euclidean graph

	for _, e := range p.mst[1:] {

		// Insert the edge between the vertices v, w.  Do this before marking the vertices.
		// CSS colors the edge gray.
		beginEdge := p.location[e.v]
		endEdge := p.location[e.w]
		lenEdge := cmplx.Abs(endEdge - beginEdge)
		distance += lenEdge
		ncells := int(columns * lenEdge / lenEP) // number of points to plot in the edge

		beginX := real(beginEdge)
		endX := real(endEdge)
		deltaX := endX - beginX
		stepX := deltaX / float64(ncells)

		beginY := imag(beginEdge)
		endY := imag(endEdge)
		deltaY := endY - beginY
		stepY := deltaY / float64(ncells)

		// loop to draw the edge
		x := beginX
		y := beginY
		for i := 0; i < ncells; i++ {
			row := int((p.ymax-y)*yscale + .5)
			col := int((x-p.xmin)*xscale + .5)
			p.plot.Grid[row*columns+col] = "edge"
			x += stepX
			y += stepY
		}

		// Mark the edge start vertex v.  CSS colors the vertex black.
		row := int((p.ymax-beginY)*yscale + .5)
		col := int((beginX-p.xmin)*xscale + .5)
		p.plot.Grid[row*columns+col] = "vertex"

		// Mark the edge end vertex w.  CSS colors the vertex black.
		row = int((p.ymax-endY)*yscale + .5)
		col = int((endX-p.xmin)*xscale + .5)
		p.plot.Grid[row*columns+col] = "vertex"
	}

	// Mark the MST start vertex.  CSS colors the vertex green.
	x := real(p.location[0])
	y := imag(p.location[0])
	p.plot.StartLocation = fmt.Sprintf("(%.2f, %.2f)", x, y)
	row := int((p.ymax-y)*yscale + .5)
	col := int((x-p.xmin)*xscale + .5)
	p.plot.Grid[row*columns+col] = "startvertexMSS"
	p.plot.Grid[(row+1)*columns+col] = "startvertexMSS"
	p.plot.Grid[(row-1)*columns+col] = "startvertexMSS"
	p.plot.Grid[row*columns+col+1] = "startvertexMSS"
	p.plot.Grid[row*columns+col-1] = "startvertexMSS"

	// Construct x-axis labels
	incr := (p.xmax - p.xmin) / (xlabels - 1)
	x = p.xmin
	// First label is empty for alignment purposes
	for i := range p.plot.Xlabel {
		p.plot.Xlabel[i] = fmt.Sprintf("%.2f", x)
		x += incr
	}

	// Construct the y-axis labels
	incr = (p.ymax - p.ymin) / (ylabels - 1)
	y = p.ymin
	for i := range p.plot.Ylabel {
		p.plot.Ylabel[i] = fmt.Sprintf("%.2f", y)
		y += incr
	}

	// Distance of the MST
	p.plot.Distance = fmt.Sprintf("%.2f", distance)

	// Endpoints and Vertices
	p.plot.Vertices = strconv.Itoa(len(p.location))
	p.plot.Xmin = fmt.Sprintf("%.2f", p.xmin)
	p.plot.Xmax = fmt.Sprintf("%.2f", p.xmax)
	p.plot.Ymin = fmt.Sprintf("%.2f", p.ymin)
	p.plot.Ymax = fmt.Sprintf("%.2f", p.ymax)

	return nil
}

// findSP constructs the shortest path from source to target
func (dsp *DijksraSP) findSP(r *http.Request) error {
	// need both source and target vertices for the shortest path
	sourceVert := r.PostFormValue("sourcevert")
	targetVert := r.PostFormValue("targetvert")
	var err error
	if len(sourceVert) == 0 || len(targetVert) == 0 {
		return fmt.Errorf("source and/or target vertices not set")
	}
	dsp.source, err = strconv.Atoi(sourceVert)
	if err != nil {
		fmt.Printf("source vertex Atoi error: %v\n", err)
		return err
	}
	dsp.target, err = strconv.Atoi(targetVert)
	if err != nil {
		fmt.Printf("target vertex Atoi error: %v\n", err)
		return err
	}

	vertices := len(dsp.location)
	if dsp.source == dsp.target || dsp.source < 0 || dsp.target < 0 ||
		dsp.source > vertices-1 || dsp.target > vertices-1 {
		return fmt.Errorf("source and/or target vertices are invalid")
	}

	dsp.edgeTo = make([]*Edge, vertices)
	dsp.distTo = make([]float64, vertices)
	for i := range dsp.distTo {
		dsp.distTo[i] = math.MaxFloat64
	}
	// Create a priority queue, put the items in it, and establish
	// the priority queue (heap) invariants.
	pq := make(PriorityQueue)

	// Create the adjacency list
	dsp.adj = make([][]*Edge, vertices)
	for i := range dsp.adj {
		dsp.adj[i] = make([]*Edge, 0)
	}
	for _, e := range dsp.mst[1:] {
		dsp.adj[e.v] = append(dsp.adj[e.v], e)
		dsp.adj[e.w] = append(dsp.adj[e.w], e)
	}

	relax := func(v int) {
		// find shortest distance from source to w
		for _, e := range dsp.adj[v] {
			// Determine v and w on the edge
			w := e.w
			if e.w == v {
				w = e.v
				e.v, e.w = e.w, e.v
			}

			newDistance := dsp.distTo[v] + dsp.graph[v][w]
			if dsp.distTo[w] > newDistance {
				// Edge to w is new best connection from source to w
				dsp.edgeTo[w] = e
				dsp.distTo[w] = dsp.distTo[v] + dsp.graph[v][w]
				// Check if already in the queue and update
				item, ok := pq[w]
				// update
				if ok {
					pq.update(item, newDistance)
				} else { // insert
					item = &Item{Edge: Edge{v: v, w: w}, distance: newDistance}
					heap.Push(&pq, item)
				}
			}
		}
	}

	// Starting index is source, distance to itself is 0, put it in the queue
	dsp.distTo[dsp.source] = 0.0
	pq[0] = &Item{index: 0, distance: 0.0, Edge: Edge{v: dsp.source, w: dsp.source}}
	heap.Init(&pq)

	// Loop until the target vertex distance is found
	for pq.Len() > 0 {
		item := heap.Pop(&pq).(*Item)
		if item.w == dsp.target {
			// empty the priority queue to avoid memory leak
			for pq.Len() > 0 {
				heap.Pop(&pq)
			}
			return nil
		}
		relax(item.w)
	}

	return nil
}

// plotSP draws the shortest path from source to target in the grid
func (dsp *DijksraSP) plotSP() error {
	// check if the target was found in findSP
	if len(dsp.distTo) == 0 || dsp.distTo[dsp.target] == math.MaxFloat64 {
		return fmt.Errorf("distance to vertex %d not found", dsp.target)
	}

	var (
		distance  float64 = 0.0
		firstEdge *Edge
	)

	// Calculate scale factors for x and y
	xscale := (columns - 1) / (dsp.xmax - dsp.xmin)
	yscale := (rows - 1) / (dsp.ymax - dsp.ymin)

	beginEP := complex(dsp.xmin, dsp.ymin) // beginning of the Euclidean graph
	endEP := complex(dsp.xmax, dsp.ymax)   // end of the Euclidean graph
	lenEP := cmplx.Abs(endEP - beginEP)    // length of the Euclidean graph

	e := dsp.edgeTo[dsp.target]
	// start at the target and loop until source vertex is plotted to the grid
	for {
		v := e.v
		w := e.w
		start := dsp.location[v]
		end := dsp.location[w]
		x1 := real(start)
		y1 := imag(start)
		x2 := real(end)
		y2 := imag(end)
		lenEdge := cmplx.Abs(end - start)
		distance += lenEdge
		ncells := int(columns * lenEdge / lenEP) // number of points to plot in the edge

		deltaX := x2 - x1
		stepX := deltaX / float64(ncells)

		deltaY := y2 - y1
		stepY := deltaY / float64(ncells)

		// loop to draw the edge; CSS colors the SP edge Yellow
		x := x1
		y := y1
		for i := 0; i < ncells; i++ {
			row := int((dsp.ymax-y)*yscale + .5)
			col := int((x-dsp.xmin)*xscale + .5)
			dsp.plot.Grid[row*columns+col] = "edgeSP"
			x += stepX
			y += stepY
		}

		// Mark the edge start vertex v.  CSS colors the vertex Black.
		row := int((dsp.ymax-y1)*yscale + .5)
		col := int((x1-dsp.xmin)*xscale + .5)
		dsp.plot.Grid[row*columns+col] = "vertex"

		// Mark the edge end vertex w.  CSS colors the vertex Black.
		row = int((dsp.ymax-y2)*yscale + .5)
		col = int((x2-dsp.xmin)*xscale + .5)
		dsp.plot.Grid[row*columns+col] = "vertex"

		// exit the loop if source is reached, we have the SP
		if e.v == dsp.source {
			firstEdge = e
			break
		}

		// move forward to the next edge
		e = dsp.edgeTo[v]
		if e.w != v {
			e.v, e.w = e.w, e.v
		}
	}

	// Mark the end vertices of the shortest path
	e = dsp.edgeTo[dsp.target]
	x := real(dsp.location[e.w])
	y := imag(dsp.location[e.w])
	// Mark the SP end vertex.  CSS colors the vertex Red.
	row := int((dsp.ymax-y)*yscale + .5)
	col := int((x-dsp.xmin)*xscale + .5)
	dsp.plot.Grid[row*columns+col] = "vertexSP2"
	dsp.plot.Grid[(row+1)*columns+col] = "vertexSP2"
	dsp.plot.Grid[(row-1)*columns+col] = "vertexSP2"
	dsp.plot.Grid[row*columns+col+1] = "vertexSP2"
	dsp.plot.Grid[row*columns+col-1] = "vertexSP2"

	dsp.plot.TargetLocation = fmt.Sprintf("(%.2f, %.2f)", x, y)
	dsp.plot.Target = strconv.Itoa(e.w)

	// Mark the SP start vertex.  CSS colors the vertex Blue.
	x = real(dsp.location[firstEdge.v])
	y = imag(dsp.location[firstEdge.v])
	row = int((dsp.ymax-y)*yscale + .5)
	col = int((x-dsp.xmin)*xscale + .5)
	dsp.plot.Grid[row*columns+col] = "vertexSP1"
	dsp.plot.Grid[(row+1)*columns+col] = "vertexSP1"
	dsp.plot.Grid[(row-1)*columns+col] = "vertexSP1"
	dsp.plot.Grid[row*columns+col+1] = "vertexSP1"
	dsp.plot.Grid[row*columns+col-1] = "vertexSP1"

	dsp.plot.SourceLocation = fmt.Sprintf("(%.2f, %.2f)", x, y)
	dsp.plot.Source = strconv.Itoa(firstEdge.v)

	// Distance of the SP
	dsp.plot.DistanceSP = fmt.Sprintf("%.2f", distance)

	return nil

}

// HTTP handler for /graphoptions connections
func handleGraphOptions(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/graphoptions.html")
}

// HTTP handler for /dijkstrasp connections
func handleDijkstraSP(w http.ResponseWriter, r *http.Request) {

	// Create the Prim MST instance
	primmst := &PrimMST{}

	// Create the Dijkstra SP instance
	dijkstrasp := &DijksraSP{}

	// Accumulate error
	status := make([]string, 0)

	// Generate V vertices and locations randomly, get from HTML form
	// or read in from a previous graph when using a new start vertex.
	// Insert vertex complex coordinates into locations
	err := primmst.generateVertices(r)
	if err != nil {
		fmt.Printf("generateVertices error: %v\n", err)
		status = append(status, err.Error())
	}

	// Insert distances into graph
	err = primmst.findDistances()
	if err != nil {
		fmt.Printf("findDistances error: %v", err)
		status = append(status, err.Error())
	}

	// Find MST and save in PrimMST.mst
	err = primmst.findMST()
	if err != nil {
		fmt.Printf("findMST error: %v\n", err)
		status = append(status, err.Error())
	}

	// Assign vertex locations to dijkstrasp so it can use x,y coordinates of vertices
	dijkstrasp.location = primmst.location
	// Assign graph to dijkstrasp so it can use distances between vertices
	dijkstrasp.graph = primmst.graph
	// Assign MST to dijkstrasp so it can use it to construct adj
	dijkstrasp.mst = primmst.mst
	// Assign endpoints to dijkstrasp for plotting on the grid
	dijkstrasp.Endpoints = primmst.Endpoints

	// Find the Shortest Path
	err = dijkstrasp.findSP(r)
	if err != nil {
		fmt.Printf("findSP error: %v\n", err)
		status = append(status, err.Error())
	}

	// Draw MST into 300 x 300 cell 2px grid
	// Construct x-axis labels, y-axis labels, status message
	err = primmst.plotMST(status)
	if err != nil {
		fmt.Printf("plotMST error: %v\n", err)
		status = append(status, err.Error())
	}

	// Assign plot to dijkstrasp
	dijkstrasp.plot = primmst.plot

	// Draw SP into 300 x 300 cell 2px grid
	err = dijkstrasp.plotSP()
	if err != nil {
		fmt.Printf("plotSP error: %v\n", err)
		status = append(status, err.Error())
	}

	// Status
	if len(status) > 0 {
		dijkstrasp.plot.Status = strings.Join(status, ", ")
	} else {
		dijkstrasp.plot.Status = "Enter Source and Target Vertices (0-V-1) for another SP"
	}

	// Write to HTTP using template and grid
	if err := tmplForm.Execute(w, primmst.plot); err != nil {
		log.Fatalf("Write to HTTP output using template with grid error: %v\n", err)
	}
}

// main sets up the http handlers, listens, and serves http clients
func main() {
	rand.Seed(time.Now().Unix())
	// Set up http servers with handler for Graph Options and Dijkstra SP
	http.HandleFunc(patternDijkstraSP, handleDijkstraSP)
	http.HandleFunc(patternGraphOptions, handleGraphOptions)
	fmt.Printf("Dijkstra Shortest Path Server listening on %v.\n", addr)
	http.ListenAndServe(addr, nil)
}
