package daemon

import (
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/nytlabs/streamtools/blocks"
	"log"
	"net/http"
	"strings"
)

var (
	// channel that returns the next ID
	idChan chan string
)

// Daemon keeps track of all the blocks and connections
type Daemon struct {
	blockMap      map[string]*blocks.Block
}

// The rootHandler returns information about the whole system
func (d *Daemon) rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "hello! this is streamtools")
	fmt.Fprintln(w, "ID: BlockType")
	for id, block := range d.blockMap {
		fmt.Fprintln(w, id+":", block.BlockType)
	}
}

// The createHandler creates new blocks
func (d *Daemon) createHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("could not parse form on /create")
	}
	if blockType, ok := r.Form["blockType"]; ok {

		var id string
		if blockId, ok := r.Form["id"]; ok {
			id = blockId[0]
		} else {
			id = <-idChan
		}
		d.CreateBlock(blockType[0], id)

	} else {
		log.Println("no blocktype specified")
	}
}

// The connectHandler connects together two blocks
func (d *Daemon) connectHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("could not parse form on /connect")
	}
	from := r.Form["from"][0]
	to := r.Form["to"][0]
	d.CreateConnection(from, to)
}

// The routeHandler deals with any incoming message sent to an arbitrary block endpoint
func (d *Daemon) routeHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.Split(r.URL.Path, "/")[2]
	route := strings.Split(r.URL.Path, "/")[3]

	err := r.ParseForm()
	var respData string
	for k, _ := range r.Form {
		respData = k
	}
	msg, err := simplejson.NewJson([]byte(respData))
	if err != nil {
		msg = nil
	}
	ResponseChan := make(chan *simplejson.Json)
	blockRouteChan := d.blockMap[id].Routes[route]
	blockRouteChan <- blocks.RouteResponse{
		Msg:          msg,
		ResponseChan: ResponseChan,
	}
	blockMsg := <-ResponseChan
	out, err := blockMsg.MarshalJSON()
	if err != nil {
		log.Println(err.Error())
	}

	fmt.Fprintln(w, string(out))
}

func (d *Daemon) libraryHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "libraryBlob")
}

func (d *Daemon) createRoutes(b *blocks.Block){
	for _, routeName := range blocks.Library[b.BlockType].RouteNames {
		log.Println("creating route /blocks/"+b.ID+"/"+routeName)
		http.HandleFunc("/blocks/"+b.ID+"/"+routeName, d.routeHandler)
	}
}

func (d *Daemon) CreateConnection(from string, to string) {
	ID := <-idChan
	d.CreateBlock("connection",ID)

	d.blockMap[from].AddChan <- &blocks.OutChanMsg{
		Action: blocks.CREATE_OUT_CHAN,
		OutChan: d.blockMap[ID].InChan,
		ID: ID,
	}

	d.blockMap[ID].AddChan <- &blocks.OutChanMsg{
		Action: blocks.CREATE_OUT_CHAN,
		OutChan: d.blockMap[to].InChan,
		ID: to,
	}
}

func (d *Daemon) CreateBlock(name string, ID string) {
	// TODO: Clean this up.
	// 
	// In order to avoid data races the blocks held in daemon's blockMap
	// are not the same blocks held in each block routine. When CreateBlock
	// is called, we actually create two blocks: one to store in daemon's
	// blockMap and one to send to the block routine.
	// 
	// The block stored in daemon's blockmap doesn't make use of OutChans as
	// a block's OutChans can be dynamically modified when connections are 
	// added or deleted. All of the other fields, such as ID, name, and all
	// the channels that go into the block (inChan, Routes) are the SAME
	// in both the daemon blockMap block and the blockroutine block.
	//
	// Becauase of this very minor difference it would be a huge semantic help
	// if the type going to the blockroutines was actually different than the 
	// type being kept in daemon's blockmap.
	//
	// Modifications to blocks in daemon's blockMap will obviously not
	// proliferate to blockroutines and all changes (such as adding outchans)
	// can only be done through messages. A future daemon block type might
	// want to restrict how daemon blocks can be used, such as creating 
	// getters and no setters. Or perhaps a setter automatically takes care
	// of sending a message to the blockroutine to emulate the manipulation
	// of a single variable.

	// create the block that will be stored in blockMap
	b, _ := blocks.NewBlock(name, ID)
	d.createRoutes(b)
	d.blockMap[b.ID] = b

	// create the block that will be sent to the blockroutine and copy all
	// chan references from the previously created block
	c, _ := blocks.NewBlock(name, ID)
	for k, v := range b.Routes {
		c.Routes[k] = v
	}
	
	c.InChan = b.InChan
	c.AddChan = b.AddChan

	//create outchans for use only by blockroutine block.
	c.OutChans = make(map[string]chan *simplejson.Json)

	go blocks.Library[name].Routine(c)
	
	log.Println("started block \"" + ID + "\" of type " + name)
}

func (d *Daemon) Run(port string) {

	// start the ID Service
	idChan = make(chan string)
	go IDService(idChan)

	// start the library service
	blocks.BuildLibrary()

	// initialise the block maps
	d.blockMap = make(map[string]*blocks.Block)

	// instantiate the base handlers
	http.HandleFunc("/", d.rootHandler)
	http.HandleFunc("/create", d.createHandler)
	http.HandleFunc("/connect", d.connectHandler)
	http.HandleFunc("/library", d.libraryHandler)

	// start the http server
	log.Println("starting stream tools on port",  port)
	err := http.ListenAndServe(":"+ port, nil)
	if err != nil {
		log.Fatalf(err.Error())
	}
}
