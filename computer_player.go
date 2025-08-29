package main

import (
	"math"
	"math/rand"
)

// GameState provides context for AI decision making
type GameState struct {
	Round         int
	Players       []PlayerInterface
	ActivePlayers []PlayerInterface
	CurrentLeader PlayerInterface
	CardsInDeck   []*Card
}

type HitOrStayStrategy func(self PlayerInterface, gameState *GameState) bool
type ActionTargetStrategy func(self PlayerInterface, gameState *GameState, actionType ActionType) PlayerInterface

type ComputerPlayer struct {
	BasePlayer
	HitOrStayStrategy            HitOrStayStrategy
	ActionTargetStrategy         ActionTargetStrategy
	PositiveActionTargetStrategy ActionTargetStrategy
}

// NewComputerPlayer creates a new computer player with specified strategy
func NewComputerPlayer(name string, strategy HitOrStayStrategy, actionTargetStrategy ActionTargetStrategy, positiveActionTargetStrategy ActionTargetStrategy) *ComputerPlayer {
	p := &ComputerPlayer{
		HitOrStayStrategy:            strategy,
		ActionTargetStrategy:         actionTargetStrategy,
		PositiveActionTargetStrategy: positiveActionTargetStrategy,
	}

	p.BasePlayer.Init(name)

	return p
}

func (p *ComputerPlayer) GetPlayerIcon() string {
	return "🤖"
}

func (p *ComputerPlayer) MakeHitStayDecision(gameState *GameState) (bool, error) {
	// Always hit if you have a second chance
	if p.HasSecondChance() {
		return true, nil
	}

	return p.HitOrStayStrategy(p, gameState), nil
}

func (p *ComputerPlayer) ChooseActionTarget(gameState *GameState, actionType ActionType) (PlayerInterface, error) {
	return p.ActionTargetStrategy(p, gameState, actionType), nil
}

func (p *ComputerPlayer) ChoosePositiveActionTarget(gameState *GameState, actionType ActionType) (PlayerInterface, error) {
	return p.PositiveActionTargetStrategy(p, gameState, actionType), nil
}

func PlayRoundTo(n int) HitOrStayStrategy {
	return func(self PlayerInterface, gameState *GameState) bool {
		return self.CalculateRoundScore() < n
	}
}

func BayesianGainStrategy(self PlayerInterface, gameState *GameState) bool {
	bustProb := CalculateLimitedKnowledgeBustProbability(self)
	currentScore := self.CalculateRoundScore()
	expectedBustCost := float64(currentScore) * bustProb

	// Approximate expected value of next number card
	// \sum(i^2)/\sum(i) for i in [1..12] = 8.33
	baseNextCardValue := 8.33

	if self.NumberOfNumberCards() > 6 {
		baseNextCardValue += 15
	}

	if hasMultiplier(self) {
		baseNextCardValue *= 2
	}

	expectedScoreIncrease := float64(baseNextCardValue) * (1 - bustProb)
	expectedNetGain := expectedScoreIncrease - expectedBustCost
	return expectedNetGain > 0.0
}

func PlayToBustProbability(p float64) HitOrStayStrategy {
	return func(self PlayerInterface, gameState *GameState) bool {
		return CalculateBustProbability(self, gameState) < p
	}
}

func CalculateLimitedKnowledgeBustProbability(player PlayerInterface) float64 {
	numberCardSum := 0
	for _, card := range player.GetHand() {
		if card.Type == NumberCard {
			numberCardSum += card.Value
		}
	}

	// 978 is \sum(i) for i in [1..12]
	return float64(numberCardSum) / float64(50)
}

// Helper functions for advanced strategies
func CalculateBustProbability(player PlayerInterface, gameState *GameState) float64 {
	numberCards := make(map[int]bool)
	for _, card := range player.GetHand() {
		if card.Type == NumberCard {
			numberCards[card.Value] = true
		}
	}

	// Count available cards that would cause a bust
	bustCards := 0
	for _, card := range gameState.CardsInDeck {
		if card.Type == NumberCard && numberCards[card.Value] {
			bustCards += 1
		}
	}

	totalCards := len(gameState.CardsInDeck)
	if totalCards == 0 {
		panic("no cards left in deck can't calculate bust probability")
	}

	return float64(bustCards) / float64(totalCards)
}

func HitUntilAheadBy(n int) HitOrStayStrategy {
	return func(self PlayerInterface, gameState *GameState) bool {
		return gameState.CurrentLeader.GetTotalScore()+gameState.CurrentLeader.CalculateRoundScore() < self.GetTotalScore()+self.CalculateRoundScore()+n
	}
}

func AlwaysHitStrategy(self PlayerInterface, gameState *GameState) bool {
	return true
}

func RandomHitOrStayStrategy(self PlayerInterface, gameState *GameState) bool {
	return rand.Intn(2) == 0
}

// Advanced strategies that could beat bust probability < 0.3

// AdaptiveBustProbabilityStrategy adjusts the bust threshold based on game state
func AdaptiveBustProbabilityStrategy(baseProbability float64) HitOrStayStrategy {
	return func(self PlayerInterface, gameState *GameState) bool {
		bustProb := CalculateBustProbability(self, gameState)

		// Adjust threshold based on position in game
		adjustedThreshold := baseProbability

		// More aggressive if behind
		if gameState.CurrentLeader != nil && gameState.CurrentLeader != self {
			leaderScore := gameState.CurrentLeader.GetTotalScore() + gameState.CurrentLeader.CalculateRoundScore()
			myScore := self.GetTotalScore() + self.CalculateRoundScore()
			gap := leaderScore - myScore

			if gap > 50 {
				adjustedThreshold += 0.15 // Much more aggressive when far behind
			} else if gap > 20 {
				adjustedThreshold += 0.1 // More aggressive when behind
			} else if gap < -20 {
				adjustedThreshold -= 0.1 // More conservative when ahead
			}
		}

		// More aggressive late in game
		if gameState.CurrentLeader != nil {
			leaderScore := gameState.CurrentLeader.GetTotalScore()
			if leaderScore > 150 {
				adjustedThreshold += 0.1 // Game is ending soon, take more risks
			}
		}

		return bustProb < adjustedThreshold
	}
}

// ExpectedValueStrategy considers the expected points gained vs risk
func ExpectedValueStrategy(self PlayerInterface, gameState *GameState) bool {
	bustProb := CalculateBustProbability(self, gameState)
	expectedPoints := CalculateExpectedPointsFromHit(self, gameState)
	currentScore := self.CalculateRoundScore()

	// Hit if expected value is positive and risk isn't too high
	expectedValue := expectedPoints * (1 - bustProb)

	// Adjust based on current position
	threshold := 2.0 // Base threshold for expected value
	if gameState.CurrentLeader != nil && gameState.CurrentLeader != self {
		leaderScore := gameState.CurrentLeader.GetTotalScore() + gameState.CurrentLeader.CalculateRoundScore()
		myScore := self.GetTotalScore() + currentScore
		if leaderScore-myScore > 30 {
			threshold = 1.0 // Lower threshold when behind
		}
	}

	return expectedValue > threshold && bustProb < 0.5
}

// HybridStrategy combines multiple factors for decision making
func HybridStrategy(self PlayerInterface, gameState *GameState) bool {
	currentScore := self.CalculateRoundScore()
	bustProb := CalculateBustProbability(self, gameState)

	// Base decision on bust probability
	baseBustThreshold := 0.25

	// Factor 1: Position relative to leader
	if gameState.CurrentLeader != nil {
		leaderScore := gameState.CurrentLeader.GetTotalScore() + gameState.CurrentLeader.CalculateRoundScore()
		myScore := self.GetTotalScore() + currentScore
		gap := leaderScore - myScore

		if gap > 40 {
			baseBustThreshold += 0.2 // Much more aggressive when far behind
		} else if gap > 15 {
			baseBustThreshold += 0.1 // More aggressive when behind
		} else if gap < -15 {
			baseBustThreshold -= 0.1 // More conservative when ahead
		}
	}

	// Factor 2: Round score - be more conservative with high scores
	if currentScore > 30 {
		baseBustThreshold -= 0.1
	} else if currentScore < 10 {
		baseBustThreshold += 0.05
	}

	// Factor 3: Game progress - more aggressive near end
	if gameState.CurrentLeader != nil {
		maxScore := gameState.CurrentLeader.GetTotalScore()
		if maxScore > 160 {
			baseBustThreshold += 0.1
		}
	}

	// Factor 4: Modifier cards - more aggressive with multipliers
	if hasMultiplier(self) {
		baseBustThreshold += 0.05
	}

	return bustProb < baseBustThreshold
}

func GapAwareStrategy(TargetScore int, GapThreshold int) HitOrStayStrategy {
	slackTarget := TargetScore - GapThreshold
	aggressiveTarget := TargetScore - GapThreshold
	slackTargetStrategy := PlayRoundTo(TargetScore)
	aggressiveTargetStrategy := PlayRoundTo(aggressiveTarget)
	normalTargetStrategy := PlayRoundTo(TargetScore)
	return func(self PlayerInterface, gameState *GameState) bool {
		score := self.CalculateRoundScore()
		if score <= aggressiveTarget {
			return aggressiveTargetStrategy(self, gameState)
		} else if score >= slackTarget {
			return slackTargetStrategy(self, gameState)
		} else {
			return normalTargetStrategy(self, gameState)
		}
	}
}

// GapBasedStrategy focuses on the score gap to other players
func GapBasedStrategy(self PlayerInterface, gameState *GameState) bool {
	if gameState.CurrentLeader == nil {
		return CalculateBustProbability(self, gameState) < 0.3
	}

	leaderScore := gameState.CurrentLeader.GetTotalScore() + gameState.CurrentLeader.CalculateRoundScore()
	myScore := self.GetTotalScore() + self.CalculateRoundScore()
	gap := leaderScore - myScore

	// Dynamic bust threshold based on gap
	var bustThreshold float64
	switch {
	case gap > 60:
		bustThreshold = 0.5 // Very aggressive when far behind
	case gap > 30:
		bustThreshold = 0.4 // Aggressive when behind
	case gap > 10:
		bustThreshold = 0.35 // Moderately aggressive
	case gap > -10:
		bustThreshold = 0.3 // Standard
	case gap > -30:
		bustThreshold = 0.25 // Conservative when ahead
	default:
		bustThreshold = 0.2 // Very conservative when far ahead
	}

	return CalculateBustProbability(self, gameState) < bustThreshold
}

// OptimalStrategy - combines best elements of gap-based and bust probability
func OptimalStrategy(self PlayerInterface, gameState *GameState) bool {
	bustProb := CalculateBustProbability(self, gameState)
	currentScore := self.CalculateRoundScore()

	// Start with dynamic gap-based threshold
	var baseThreshold float64
	if gameState.CurrentLeader != nil {
		leaderScore := gameState.CurrentLeader.GetTotalScore() + gameState.CurrentLeader.CalculateRoundScore()
		myScore := self.GetTotalScore() + currentScore
		gap := leaderScore - myScore

		switch {
		case gap > 50:
			baseThreshold = 0.45 // Very aggressive when far behind
		case gap > 25:
			baseThreshold = 0.35 // Aggressive when behind
		case gap > 10:
			baseThreshold = 0.3 // Moderately aggressive
		case gap > -10:
			baseThreshold = 0.28 // Slightly conservative
		case gap > -25:
			baseThreshold = 0.25 // Conservative when ahead
		default:
			baseThreshold = 0.22 // Very conservative when far ahead
		}
	} else {
		baseThreshold = 0.3 // Default when no leader
	}

	// Adjust for current round score (be more conservative with high scores)
	if currentScore > 35 {
		baseThreshold -= 0.08
	} else if currentScore > 25 {
		baseThreshold -= 0.05
	} else if currentScore < 10 {
		baseThreshold += 0.03
	}

	// Adjust for game state
	if gameState.CurrentLeader != nil {
		maxScore := gameState.CurrentLeader.GetTotalScore()
		if maxScore > 170 {
			baseThreshold += 0.05 // More aggressive near end of game
		} else if maxScore > 150 {
			baseThreshold += 0.03
		}
	}

	// Adjust for modifier cards
	if hasMultiplier(self) && currentScore < 25 {
		baseThreshold += 0.04 // More aggressive with multiplier at low scores
	}

	// Apply minimum and maximum bounds
	if baseThreshold > 0.5 {
		baseThreshold = 0.5
	} else if baseThreshold < 0.15 {
		baseThreshold = 0.15
	}

	return bustProb < baseThreshold
}

func CalculateExpectedPointsFromHit(player PlayerInterface, gameState *GameState) float64 {
	numberCards := make(map[int]bool)
	for _, card := range player.GetHand() {
		if card.Type == NumberCard {
			numberCards[card.Value] = true
		}
	}

	totalPoints := 0.0
	validCards := 0

	for _, card := range gameState.CardsInDeck {
		if card.Type == NumberCard && !numberCards[card.Value] {
			totalPoints += float64(card.Value)
			validCards++
		} else if card.Type == ModifierCard {
			totalPoints += float64(card.GetPoints())
			validCards++
		}
		// Action cards have variable value, approximate as 5 points
		if card.Type == ActionCard {
			totalPoints += 5.0
			validCards++
		}
	}

	if validCards == 0 {
		return 0
	}

	return totalPoints / float64(validCards)
}

func hasMultiplier(player PlayerInterface) bool {
	for _, card := range player.GetHand() {
		if card.Type == ModifierCard && card.Modifier == Multiply2 {
			return true
		}
	}
	return false
}

func TargetLeaderStrategy(self PlayerInterface, gameState *GameState, actionType ActionType) PlayerInterface {
	var leader PlayerInterface
	leaderScore := 0
	for _, player := range gameState.ActivePlayers {
		if actionType == SecondChance && player.HasSecondChance() {
			continue
		}

		if player != self {
			playerScore := player.GetTotalScore() + player.CalculateRoundScore()
			if playerScore > leaderScore {
				leader = player
				leaderScore = playerScore
			}
		}
	}

	// Must target self if no other player is active
	if leader == nil {
		return self
	}

	return leader
}

func TargetLastPlaceStrategy(self PlayerInterface, gameState *GameState, actionType ActionType) PlayerInterface {
	var last PlayerInterface
	lastScore := math.MaxInt
	for _, player := range gameState.ActivePlayers {
		if actionType == SecondChance && player.HasSecondChance() {
			continue
		}

		if player != self {
			playerScore := player.GetTotalScore() + player.CalculateRoundScore()
			if playerScore < lastScore {
				last = player
				lastScore = playerScore
			}
		}
	}

	// Must target self if no other player is active
	if last == nil {
		return self
	}

	return last
}

func TargetRandomStrategy(self PlayerInterface, gameState *GameState, actionType ActionType) PlayerInterface {
	activePlayers := make([]PlayerInterface, 0)
	for _, player := range gameState.Players {
		if actionType == SecondChance && player.HasSecondChance() {
			continue
		}

		if player.IsActive() && player != self {
			activePlayers = append(activePlayers, player)
		}
	}

	// Must target self if no other player is active
	if len(activePlayers) == 0 {
		return self
	}

	return activePlayers[rand.Intn(len(activePlayers))]
}
