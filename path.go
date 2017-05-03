package svg

import (
	"fmt"
	"strconv"

	mt "github.com/rustyoz/Mtransform"
)

type Path struct {
	Id          string `xml:"id,attr"`
	D           string `xml:"d,attr"`
	Style       string `xml:"style,attr"`
	properties  map[string]string
	strokeWidth float64
	command     chan *Command
	group       *Group
}

//
const (
	MOVETO = iota
	LINETO
	HLINETO
	VLINETO
	CURVETO
	CLOSEPATH
)

// Command is
type Command struct {
	Name    int
	Points  []Tuple
	Numbers []float64
}

func (c *Command) addPoint(p Tuple) {
	c.Points = append(c.Points, p)
}

// Segment
// A segment of a path that contains a list of connected points, its stroke Width and if the segment forms a closed loop.
// Points are defined in world space after any matrix transformation is applied.
type Segment struct {
	Width  float64
	Closed bool
	Points [][2]float64
}

func (p Path) newSegment(start [2]float64) *Segment {
	var s Segment
	s.Width = p.strokeWidth * p.group.Owner.scale
	s.Points = append(s.Points, start)
	return &s
}

func (s *Segment) addPoint(p [2]float64) {
	s.Points = append(s.Points, p)
}

type pathDescriptionParser struct {
	p              *Path
	lex            Lexer
	x, y           float64
	currentcommand int
	tokbuf         [4]Item
	peekcount      int
	lasttuple      Tuple
	transform      mt.Transform
	svg            *Svg
	currentsegment *Segment
}

func newPathDParse() *pathDescriptionParser {
	pdp := &pathDescriptionParser{}
	pdp.transform = mt.Identity()
	return pdp
}

// Parse interprets path description, transform and style atttributes to create a channel of segments.
func (p *Path) Parse() chan *Command {
	p.parseStyle()
	pdp := newPathDParse()
	pdp.p = p
	pdp.svg = p.group.Owner
	pdp.transform.MultiplyWith(*p.group.Transform)
	p.command = make(chan *Command)
	l, _ := Lex(fmt.Sprint(p.Id), p.D)
	pdp.lex = *l
	go func() {
		defer close(p.command)
		for {
			i := pdp.lex.NextItem()
			switch {
			case i.Type == ItemError:
				return
			case i.Type == ItemEOS:
				return
			case i.Type == ItemLetter:
				parseCommand(pdp, l, i)
			default:
			}
		}
	}()
	return p.command
}

func parseCommand(pdp *pathDescriptionParser, l *Lexer, i Item) error {
	var err error
	switch i.Value {
	case "M":
		err = parseMoveToAbs(pdp)
	case "m":
		err = parseMoveToRel(pdp)
	case "c":
		err = parseCurveToRel(pdp)
	case "C":
		err = parseCurveToAbs(pdp)
	case "L":
		err = parseLineToAbs(pdp)
	case "l":
		err = parseLineToRel(pdp)
	case "H":
		err = parseHLineToAbs(pdp)
	case "h":
		err = parseHLineToRel(pdp)
	case "V":
		err = parseVLineToAbs(pdp)
	case "v":
		err = parseVLineToRel(pdp)
	case "Z":
		err = parseClose(pdp)
	case "z":
		err = parseClose(pdp)
	default:
		fmt.Println("didn't parse", i.Value)
	}
	if err != nil {
		fmt.Println("parse command error", err)
	}
	return err

}

func parseMoveToAbs(pdp *pathDescriptionParser) error {
	tuples, err := parseTuples(pdp, true)
	if err != nil {
		return err
	}
	cmd := &Command{
		Name: MOVETO,
	}
	cmd.Points = tuples
	pdp.x = tuples[len(tuples)-1][0]
	pdp.y = tuples[len(tuples)-1][1]
	pdp.p.command <- cmd
	return nil
}

func parseLineToAbs(pdp *pathDescriptionParser) error {
	tuples, err := parseTuples(pdp, true)
	if err != nil {
		return err
	}
	cmd := &Command{
		Name: LINETO,
	}
	cmd.Points = tuples
	pdp.x = tuples[len(tuples)-1][0]
	pdp.y = tuples[len(tuples)-1][1]
	pdp.p.command <- cmd
	return nil
}

func parseMoveToRel(pdp *pathDescriptionParser) error {
	tuples, err := parseTuples(pdp, false)
	if err != nil {
		return err
	}
	cmd := &Command{
		Name: MOVETO,
	}
	cmd.Points = tuples
	pdp.x = tuples[len(tuples)-1][0]
	pdp.y = tuples[len(tuples)-1][1]
	pdp.p.command <- cmd
	return nil
}

func parseLineToRel(pdp *pathDescriptionParser) error {
	tuples, err := parseTuples(pdp, false)
	if err != nil {
		return err
	}
	cmd := &Command{
		Name: LINETO,
	}
	cmd.Points = tuples
	pdp.x = tuples[len(tuples)-1][0]
	pdp.y = tuples[len(tuples)-1][1]
	pdp.p.command <- cmd
	return nil
}

func parseNumbers(pdp *pathDescriptionParser) ([]float64, error) {
	var numbers []float64
	for pdp.lex.PeekItem().Type == ItemNumber {
		t, err := parseNumber(pdp.lex.NextItem())
		if err != nil {
			return nil, err
		}
		numbers = append(numbers, t)
	}
	return numbers, nil
}

func parseTuples(pdp *pathDescriptionParser, abs bool) ([]Tuple, error) {
	var tuples []Tuple
	pdp.lex.ConsumeWhiteSpace()
	for pdp.lex.PeekItem().Type == ItemNumber {
		t, err := parseTuple(&pdp.lex)
		if err != nil {
			return nil, err
		}
		if !abs {
			t[0] += pdp.x
			t[1] += pdp.y
		}
		tuples = append(tuples, t)
		pdp.lex.ConsumeWhiteSpace()
	}
	return tuples, nil
}

func parseHLineToAbs(pdp *pathDescriptionParser) error {
	numbers, err := parseNumbers(pdp)
	if err != nil {
		return err
	}
	cmd := &Command{
		Name: HLINETO,
	}
	cmd.Numbers = numbers
	pdp.x = numbers[len(numbers)-1]
	pdp.p.command <- cmd
	return nil
}

func parseHLineToRel(pdp *pathDescriptionParser) error {
	numbers, err := parseNumbers(pdp)
	if err != nil {
		return err
	}
	cmd := &Command{
		Name: HLINETO,
	}
	for i := range numbers {
		numbers[i] += pdp.x
	}
	cmd.Numbers = numbers
	pdp.x = numbers[len(numbers)-1]
	pdp.p.command <- cmd
	return nil
}

func parseVLineToAbs(pdp *pathDescriptionParser) error {
	numbers, err := parseNumbers(pdp)
	if err != nil {
		return err
	}
	cmd := &Command{
		Name: VLINETO,
	}
	cmd.Numbers = numbers
	pdp.y = numbers[len(numbers)-1]
	pdp.p.command <- cmd
	return nil
}

func parseClose(pdp *pathDescriptionParser) error {
	pdp.p.command <- &Command{
		Name: CLOSEPATH,
	}
	return nil
}

func parseVLineToRel(pdp *pathDescriptionParser) error {
	numbers, err := parseNumbers(pdp)
	if err != nil {
		return err
	}
	cmd := &Command{
		Name: VLINETO,
	}
	for i := range numbers {
		numbers[i] += pdp.x
	}
	cmd.Numbers = numbers
	pdp.y = numbers[len(numbers)-1]
	pdp.p.command <- cmd
	return nil
}

func parseCurveToRel(pdp *pathDescriptionParser) error {
	tuples, err := parseTuples(pdp, false)
	if err != nil {
		return err
	}
	cmd := &Command{
		Name: CURVETO,
	}
	cmd.Points = tuples
	pdp.x = tuples[len(tuples)-1][0]
	pdp.y = tuples[len(tuples)-1][1]
	pdp.p.command <- cmd
	return nil
}

func parseCurveToAbs(pdp *pathDescriptionParser) error {
	tuples, err := parseTuples(pdp, true)
	if err != nil {
		return err
	}
	cmd := &Command{
		Name: CURVETO,
	}
	cmd.Points = tuples
	pdp.x = tuples[len(tuples)-1][0]
	pdp.y = tuples[len(tuples)-1][1]
	pdp.p.command <- cmd
	return nil
}

func (p *Path) parseStyle() {
	if p.Style == "" {
		return
	}
	p.properties = splitStyle(p.Style)
	for key, val := range p.properties {
		switch key {
		case "stroke-width":
			sw, ok := strconv.ParseFloat(val, 64)
			if ok == nil {
				p.strokeWidth = sw
			}

		}
	}
}
