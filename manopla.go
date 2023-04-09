package main

import (
	"embed"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

type move struct {
	name  string
	front bool
	back  bool
	leg   bool

	// a probabilidade é um valor percentual inverso, para evitar
	// de precisar preenchê-la para cada `move`.
	// 0.0 significa que o movimento pode sempre aparecer, enquanto 1.0
	// significa que o movimento nunca poderá aparecer
	prob float64
}

//go:embed sound
var fs embed.FS

var possibleMoves = []move{
	{name: "jab", front: true, back: false},
	{name: "direto", front: false, back: true},
	{name: "cruza", front: true, back: true, prob: 0.5},
	{name: "chuta", front: true, back: true, leg: true},
	{name: "tip", front: true, back: true, leg: true, prob: 0.5},
	{name: "upper", front: true, back: true, prob: 0.5},
	{name: "cotovelo", front: true, back: true, prob: 0.5},
	{name: "joelho", front: true, back: true, leg: true},
}

var level int
var count int
var interval time.Duration

func main() {
	flag.IntVar(&level, "l", 0, "Nível de dificuldade dos movimentos. '0' (padrão) habilita todos os movimentos")
	flag.IntVar(&count, "n", 2, "Número de movimentos por série. Padrão: 2")
	flag.DurationVar(&interval, "t", 1*time.Second, "Intervalo entre as séries")
	flag.Parse()
	rand.Seed(2)

	sr := beep.SampleRate(48000)
	err := speaker.Init(sr, sr.N(time.Second/10))
	if err != nil {
		panic(err)
	}

	isFront := true

	if level != 0 {
		if level+1 >= len(possibleMoves) {
			level = len(possibleMoves) - 2
		}
		possibleMoves = possibleMoves[:level+1]
	}

	for {
		moves := []move{}
		s := ""
		if isFront {
			s += "F:"
		} else {
			s += "T:"
		}

		for i := 0; i < count; i++ {
			move := nextMove(moves, count, isFront)
			moves = append(moves, move)
			isFront = !isFront
			s += " " + move.name
		}

		fmt.Println(s)

		for _, m := range moves {
			err := playAudio(m.name)
			if err != nil {
				panic(err)
			}
		}

		time.Sleep(interval)
	}
}

func nextMove(moves []move, count int, isFront bool) move {
	type rule func(move) bool

	rules := []rule{}

	rules = append(rules, func(m move) bool {
		p := rand.Float64()
		return m.prob <= p
	})

	// somente movimentos permitidos para aquele lado, ex:
	// jab só na frente, direto só com o braço de trás
	if isFront {
		rules = append(rules, func(m move) bool {
			return m.front
		})
	} else {
		rules = append(rules, func(m move) bool {
			return m.back
		})
	}

	hasLeg := false
	for _, m := range possibleMoves {
		if m.leg {
			hasLeg = true
			break
		}
	}

	if hasLeg {
		// exigir perna somente para *ultimo* golpe quando forem 4 ou mais movimentos
		// caso contrário, proibir perna caso não seja uma série de 1 só movimento
		if count >= 4 && len(moves) == count-1 {
			rules = append(rules, func(m move) bool {
				return m.leg
			})
		} else if count != 1 {
			rules = append(rules, func(m move) bool {
				return !m.leg
			})
		}
	}

	allowedMoves := []move{}

	// filtra os movimentos de acordo com as regras
	for _, m := range possibleMoves {
		pass := true
		for _, r := range rules {
			if !r(m) {
				pass = false
				break
			}
		}

		if pass {
			allowedMoves = append(allowedMoves, m)
		}
	}

	i := rand.Intn(len(allowedMoves))
	return allowedMoves[i]
}

func playAudio(name string) error {
	f, err := fs.Open("sound/" + name + ".mp3")
	if err != nil {
		return err
	}
	defer f.Close()

	streamer, _, err := mp3.Decode(f)
	if err != nil {
		return err
	}
	defer streamer.Close()

	done := make(chan struct{})

	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- struct{}{}
	})))

	<-done

	return nil
}
