package blocks

import (
	"github.com/nytlabs/gojee" // jee
	"log"
	"math"
)

// Sd calculates standard deviation in an online fashion using Welford's Algorithm.
// Ref: http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.302.7503&rep=rep1&type=pdf
func Sd(b *Block) {

	type sdRule struct {
		Key string
	}

	type sdData struct {
		StDev float64
	}

	data := &sdData{StDev: 0.0}
	var rule *sdRule
	var tree *jee.TokenTree

	N := 0.0
	M_curr := 0.0
	M_new := 0.0
	S := 0.0

	for {
		select {
		case query := <-b.Routes["sd"]:
			marshal(query, data)
		case ruleUpdate := <-b.Routes["set_rule"]:
			if rule == nil {
				rule = &sdRule{}
			}
			unmarshal(ruleUpdate, rule)
			token, err := jee.Lexer(rule.Key)
			if err != nil {
				log.Println(err.Error())
				break
			}
			tree, err = jee.Parser(token)
			if err != nil {
				log.Println(err.Error())
				break
			}

		case msg := <-b.Routes["get_rule"]:
			if rule == nil {
				marshal(msg, &sdRule{})
			} else {
				marshal(msg, rule)
			}
		case <-b.QuitChan:
			quit(b)
			return
		case msg := <-b.InChan:
			if rule == nil {
				break
			}
			if tree == nil {
				break
			}
			val, err := jee.Eval(tree, msg.Msg)
			if err != nil {
				log.Println(err.Error())
				break
			}

			x, ok := val.(float64)
			if !ok {
				break
			}
			N++
			if N == 1.0 {
				M_curr = x
			} else {
				M_new = M_curr + (x-M_curr)/N
				S = S + (x-M_curr)*(x-M_new)
				M_curr = M_new
			}
			data.StDev = math.Sqrt(S / (N - 1.0))
		}
	}
}
