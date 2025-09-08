package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ComputerPlayerConfig Config structs for game_config.json
type ComputerPlayerConfig struct {
	Name           string `json:"name"`
	StrategyChoice int    `json:"strategy_choice"`
}

type GameConfig struct {
	TotalPlayers    int                    `json:"total_players"`
	HumanPlayers    int                    `json:"human_players"`
	HumanNames      []string               `json:"human_names"`
	ComputerPlayers []ComputerPlayerConfig `json:"computer_players"`
}

// Game represents the main game state
type Game struct {
	players    []PlayerInterface
	deck       *Deck
	round      int
	dealerIdx  int
	scanner    *bufio.Scanner
	debugMode  bool
	silentMode bool
	config     *GameConfig
}

// NewGame creates a new Flip 7 game instance
func NewGame() *Game {
	g := &Game{
		players:   make([]PlayerInterface, 0),
		deck:      NewDeck(),
		round:     1,
		scanner:   bufio.NewScanner(os.Stdin),
		debugMode: false,
		config:    nil,
	}
	g.loadConfig()
	return g
}

// loadConfig tries to load setup from game_config.json
func (g *Game) loadConfig() {
	configFile := "game_config.json"
	data, err := os.ReadFile(configFile)
	if err != nil {
		// Config file not found is not an error, just proceed with interactive setup
		return
	}

	// Do not proceed if the file is empty
	if len(data) == 0 {
		return
	}

	var config GameConfig
	if err := json.Unmarshal(data, &config); err != nil {
		g.printf("Warning: Could not parse %s, proceeding with interactive setup. Error: %v\n", configFile, err)
		return
	}

	// If total_players is 0, assume config is a placeholder and ignore it
	if config.TotalPlayers <= 0 {
		return
	}

	// Basic validation
	if config.TotalPlayers != config.HumanPlayers+len(config.ComputerPlayers) {
		g.printf("Warning: Mismatch in player counts in %s, proceeding with interactive setup.\n", configFile)
		return
	}

	g.config = &config
	g.printf("✅ Loaded configuration from %s\n", configFile)
}

// SetDebugMode enables or disables debug mode
func (g *Game) SetDebugMode(debug bool) {
	g.debugMode = debug
	g.deck.SetDebugMode(debug, g.scanner)
}

// SetSilentMode enables or disables silent mode (no output)
func (g *Game) SetSilentMode(silent bool) {
	g.silentMode = silent
}

// printf prints formatted output only when not in silent mode
func (g *Game) printf(format string, args ...interface{}) {
	if !g.silentMode {
		fmt.Printf(format, args...)
	}
}

// println prints output only when not in silent mode
func (g *Game) println(args ...interface{}) {
	if !g.silentMode {
		fmt.Println(args...)
	}
}

// print prints output only when not in silent mode
func (g *Game) print(args ...interface{}) {
	if !g.silentMode {
		fmt.Print(args...)
	}
}

// Run starts the main game loop
func (g *Game) Run() error {
	// Setup players
	if err := g.setupPlayers(); err != nil {
		return err
	}

	g.println("\n🎮 Starting Flip 7! First to 200 points wins!")

	// Main game loop
	for !g.hasWinner() {
		g.printf("\n" + strings.Repeat("=", 50))
		g.printf("\n🎯 ROUND %d\n", g.round)
		g.printf(strings.Repeat("=", 50) + "\n")

		if err := g.playRound(); err != nil {
			return err
		}

		g.showScores()

		// Pause for user to continue, but not in silent/simulation mode
		if !g.silentMode {
			g.printf("\nPress [Enter] to continue to the next round...")
			g.scanner.Scan()
		}

		g.nextRound()
	}

	winner := g.getWinner()
	g.printf("\n🎉 GAME OVER! %s wins with %d points! 🎉\n", winner.GetName(), winner.GetTotalScore())

	return nil
}

// Helper methods for input handling
func (g *Game) getIntInput(min, max int) (int, error) {
	for {
		if !g.scanner.Scan() {
			return 0, fmt.Errorf("failed to read input")
		}

		input := strings.TrimSpace(g.scanner.Text())
		num, err := strconv.Atoi(input)
		if err != nil {
			g.printf("Please enter a valid number between %d and %d: ", min, max)
			continue
		}

		if num < min || num > max {
			g.printf("Please enter a number between %d and %d: ", min, max)
			continue
		}

		return num, nil
	}
}

func (g *Game) getStringInput() (string, error) {
	if !g.scanner.Scan() {
		return "", fmt.Errorf("failed to read input")
	}
	return strings.TrimSpace(g.scanner.Text()), nil
}

func (g *Game) hasWinner() bool {
	for _, player := range g.players {
		if player.GetTotalScore() >= 200 {
			return true
		}
	}
	return false
}

func (g *Game) getWinner() PlayerInterface {
	var winner PlayerInterface
	maxScore := -1

	for _, player := range g.players {
		if player.GetTotalScore() > maxScore {
			maxScore = player.GetTotalScore()
			winner = player
		}
	}

	return winner
}

func (g *Game) showScores() {
	g.println("\n📊 Current Scores:")
	g.println(strings.Repeat("-", 40))
	for _, player := range g.players {
		icon := player.GetPlayerIcon()
		g.printf("%s %-20s: %3d points\n", icon, player.GetName(), player.GetTotalScore())
	}
	g.println(strings.Repeat("-", 40))
}

func (g *Game) nextRound() {
	g.round++
	g.dealerIdx = (g.dealerIdx + 1) % len(g.players)

	// Reset players for new round
	for _, player := range g.players {
		discardedCards := player.ResetForNewRound()
		for _, card := range discardedCards {
			g.deck.DiscardCard(card)
		}
	}

	totalCards := g.deck.CardsLeft() + len(g.deck.discards)
	for _, player := range g.players {
		totalCards += len(player.GetHand())
	}

	if totalCards != g.deck.OriginalTotal {
		totals := map[string]int{}
		for _, card := range g.deck.cards {
			totals[card.String()]++
		}
		for _, card := range g.deck.discards {
			totals[card.String()]++
		}
		for _, player := range g.players {
			for _, card := range player.GetHand() {
				totals[card.String()]++
			}
		}
		g.println(totals)
		panic(fmt.Sprintf("Total cards is not the original total. Cards are disappearing! found: %d != excpected: %d", totalCards, g.deck.OriginalTotal))
	}
}

func (g *Game) playRound() error {
	g.printf("Dealer: %s\n\n", g.players[g.dealerIdx].GetName())

	// Deal initial cards
	if err := g.dealInitialCards(); err != nil {
		return err
	}

	// Play turns until round ends
	if err := g.playTurns(); err != nil {
		return err
	}

	// Calculate scores
	g.calculateRoundScores()

	return nil
}

func (g *Game) dealInitialCards() error {
	g.println("🃏 Dealing initial cards...")

	// Deal one card to each player
	for i := 0; i < len(g.players); i++ {
		playerIdx := (g.dealerIdx + 1 + i) % len(g.players)
		player := g.players[playerIdx]

		// Could have busted because of an action card
		if !player.IsActive() {
			continue
		}

		card := g.deck.DrawCard()
		if card == nil {
			return fmt.Errorf("deck is empty")
		}

		g.printf("   %s draws %s\n", player.GetName(), card.String())

		// Handle action cards immediately
		if card.IsActionCard() {
			if err := g.handleActionCard(player, card); err != nil {
				return err
			}
		} else {
			if err := player.AddCard(card); err != nil {
				return g.handleCardAddError(player, card, err)
			}
		}
	}

	g.println()
	g.showAllHands()
	return nil
}

func (g *Game) playTurns() error {
	for g.hasActivePlayers() {
		for i := 0; i < len(g.players); i++ {
			playerIdx := (g.dealerIdx + 1 + i) % len(g.players)
			player := g.players[playerIdx]

			if !player.IsActive() {
				continue
			}

			// Player must hit if they have no number cards
			if !player.HasCards() {
				g.printf("🎯 %s has no number cards and must HIT\n", player.GetName())
				if err := g.playerHit(player); err != nil {
					return err
				}
				continue
			}

			// Ask player to hit or stay
			choice, err := g.getPlayerChoice(player)
			if err != nil {
				return err
			}

			if choice == "h" {
				if err := g.playerHit(player); err != nil {
					return err
				}
			} else {
				g.playerStay(player)
			}

			if !g.hasActivePlayers() {
				break
			}
		}
	}

	return nil
}

func (g *Game) calculateRoundScores() {
	g.println("📊 Calculating round scores...")
	g.println(strings.Repeat("-", 40))

	for _, player := range g.players {
		roundScore := player.CalculateRoundScore()
		player.AddToTotalScore()

		g.printf("%s: %d points this round (Total: %d)\n",
			player.GetName(), roundScore, player.GetTotalScore())
	}
	g.println(strings.Repeat("-", 40))
}

// Helper methods for gameplay

func (g *Game) hasActivePlayers() bool {
	for _, player := range g.players {
		if player.IsActive() {
			return true
		}
	}
	return false
}

func (g *Game) showAllHands() {
	if g.silentMode {
		return
	}

	for _, player := range g.players {
		player.ShowHand()
	}
}

func (g *Game) getPlayerChoice(player PlayerInterface) (string, error) {
	gameState := g.buildGameState()
	shouldHit, err := player.MakeHitStayDecision(gameState)
	if err != nil {
		return "", err
	}

	if shouldHit {
		return "h", nil
	} else {
		return "s", nil
	}
}

func (g *Game) playerHit(player PlayerInterface) error {
	card := g.deck.DrawCard()
	if card == nil {
		return fmt.Errorf("deck is empty")
	}

	g.printf("   %s draws %s\n", player.GetName(), card.String())

	if card.IsActionCard() {
		return g.handleActionCard(player, card)
	}

	if err := player.AddCard(card); err != nil {
		return g.handleCardAddError(player, card, err)
	}
	fmt.Printf("%s's hand: %v (Score: %d)\n", player.GetName(), player.GetHand(), player.CalculateRoundScore())
	println("\n============Another Player Action============")

	return nil
}

func (g *Game) playerStay(player PlayerInterface) {
	player.Stay()
	player.CalculateRoundScore()
	g.printf("   %s stays with %d points\n", player.GetName(), player.CalculateRoundScore())
}

func (g *Game) handleActionCard(player PlayerInterface, card *Card) error {
	g.printf("   🎲 Action card! %s\n", card.String())

	switch card.Action {
	case Freeze:
		return g.handleFreezeCard(player, card)
	case FlipThree:
		return g.handleFlipThreeCard(player, card)
	case SecondChance:
		return g.handleSecondChanceCard(player, card)
	}

	return nil
}

func (g *Game) handleFreezeCard(player PlayerInterface, card *Card) error {
	target, err := g.chooseActionTarget(player, "Who should be frozen?", Freeze)
	if err != nil {
		g.deck.DiscardCard(card) // Discard card even if target selection fails
		return err
	}

	target.Stay()
	target.CalculateRoundScore()
	g.printf("   ❄️ %s is frozen and stays with %d points!\n", target.GetName(), target.CalculateRoundScore())

	g.deck.DiscardCard(card)
	return nil
}

func (g *Game) handleFlipThreeCard(player PlayerInterface, card *Card) error {
	target, err := g.chooseActionTarget(player, "Who should flip three cards?", FlipThree)
	if err != nil {
		g.deck.DiscardCard(card) // Discard card even if target selection fails
		return err
	}

	g.printf("   🎲 %s must flip 3 cards!\n", target.GetName())

	for i := 0; i < 3; i++ {
		if !target.IsActive() {
			break
		}

		drawnCard := g.deck.DrawCard()
		if drawnCard == nil {
			break
		}

		g.printf("      Card %d: %s\n", i+1, drawnCard.String())

		if drawnCard.IsActionCard() {
			// Handle nested action cards after all 3 cards are drawn
			if err := g.handleActionCard(target, drawnCard); err != nil {
				if strings.Contains(err.Error(), "flip7") {
					g.printf("   🎉 %s achieved FLIP 7!\n", target.GetName())
					g.endRoundForFlip7(target)
					break // End the Flip Three loop
				}
				// Discard the action card if there was an error handling it
				g.deck.DiscardCard(drawnCard)
				return err
			}
		} else {
			if err := target.AddCard(drawnCard); err != nil {
				if err := g.handleCardAddError(target, drawnCard, err); err != nil {
					return err
				}
				break
			}
		}
	}

	g.deck.DiscardCard(card)
	return nil
}

func (g *Game) handleSecondChanceCard(player PlayerInterface, card *Card) error {
	// Try to give it to the player who drew it first
	if !player.HasSecondChance() {
		g.printf("   🆘 %s receives a Second Chance card!\n", player.GetName())
		if err := player.AddCard(card); err != nil {
			g.deck.DiscardCard(card)
			return err
		}
		return nil
	}

	// Player already has second chance, need to give it to someone else
	g.printf("   🆘 %s already has Second Chance, must give to another player\n", player.GetName())
	target, err := player.ChoosePositiveActionTarget(g.buildGameState(), SecondChance)
	if err != nil {
		g.printf("   🆘 No one can take the Second Chance card, discarding\n")
		g.deck.DiscardCard(card)
		return nil
	}

	if err := target.AddCard(card); err != nil {
		g.deck.DiscardCard(card)
		g.printf("   🆘 %s cannot take the Second Chance card, discarding\n", player.GetName())
		return nil
	}

	g.printf("   🆘 %s receives a Second Chance card!\n", target.GetName())
	return nil
}

func (g *Game) chooseActionTarget(player PlayerInterface, prompt string, actionType ActionType) (PlayerInterface, error) {
	gameState := g.buildGameState()
	return player.ChooseActionTarget(gameState, actionType)
}

func (g *Game) handleCardAddError(player PlayerInterface, card *Card, err error) error {
	if strings.Contains(err.Error(), "flip7") {
		g.printf("   🎉 %s achieved FLIP 7 and wins the round!\n", player.GetName())
		// Mark all other players as non-active to end the round
		g.endRoundForFlip7(player)
		return nil // Don't propagate the error, just end the round
	}

	if strings.Contains(err.Error(), "duplicate_with_second_chance") {
		g.printf("   💥 %s drew a duplicate %s but has Second Chance!\n", player.GetName(), card)
		secondChanceCard := player.UseSecondChance()
		g.deck.DiscardCard(secondChanceCard) // Discard the second chance card
		g.deck.DiscardCard(card)             // Discard the duplicate
		return nil
	}

	if strings.Contains(err.Error(), "bust") {
		g.deck.DiscardCard(card) // Discard the duplicate
		g.printf("   💥 %s busts and is out of the round!\n", player.GetName())
		return nil
	}

	if strings.Contains(err.Error(), "second_chance_duplicate") {
		newTarget, err := player.ChoosePositiveActionTarget(g.buildGameState(), SecondChance)
		if err != nil {
			return err
		}

		if err := newTarget.AddCard(card); err != nil {
			// Can't give second chance to anyone
			g.printf("   🆘 %s cannot give the Second Chance card to anyone, discarding\n", player.GetName())
			g.deck.DiscardCard(card)
			return nil
		} else {
			g.printf("   🆘 %s gives the Second Chance card to %s\n", player.GetName(), newTarget.GetName())
		}

		return nil
	}

	return err
}

// setupPlayers handles the initial player setup (human vs computer)
func (g *Game) setupPlayers() error {
	if g.config != nil && g.config.TotalPlayers > 0 {
		return g.setupPlayersFromConfig()
	}
	return g.setupPlayersInteractively()
}

func (g *Game) setupPlayersFromConfig() error {
	numHumans := g.config.HumanPlayers
	numComputers := len(g.config.ComputerPlayers)

	// Setup human players
	for i := 0; i < numHumans; i++ {
		var name string
		var err error

		// Try to get name from config
		if i < len(g.config.HumanNames) && g.config.HumanNames[i] != "" {
			name = g.config.HumanNames[i]
		} else {
			g.printf("Enter name for Human Player %d: ", i+1)
			name, err = g.getStringInput()
			if err != nil {
				name = fmt.Sprintf("Human %d", i+1)
			}
		}

		g.players = append(g.players, NewHumanPlayer(name, g.scanner))
	}

	// Setup computer players from config
	for _, compConfig := range g.config.ComputerPlayers {
		name := compConfig.Name
		strategy, actionTarget, positiveTarget := getStrategyByChoice(compConfig.StrategyChoice, &name)
		g.players = append(g.players, NewComputerPlayer(name, strategy, actionTarget, positiveTarget))
		g.printf("  → Added configured AI: %s\n", name)
	}

	if numHumans == 0 {
		g.printf("\n🎮 Starting AI-only Flip 7 with %d computer players from config!\n", numComputers)
		// Note: AI-only simulation mode (runMultipleGames) is only available in interactive setup.
		// Config-based setup will run a single game.
	} else {
		g.printf("\n🎮 Starting Flip 7 with %d humans and %d computers from config!\n", numHumans, numComputers)
	}

	return nil
}

func (g *Game) setupPlayersInteractively() error {
	g.println("How many players total? (2-18): ")
	numPlayers, err := g.getIntInput(2, 18)
	if err != nil {
		return err
	}

	g.printf("How many human players? (0-%d): ", numPlayers)
	numHumans, err := g.getIntInput(0, numPlayers)
	if err != nil {
		return err
	}

	numComputers := numPlayers - numHumans

	// Setup human players
	for i := 0; i < numHumans; i++ {
		g.printf("Enter name for Human Player %d: ", i+1)
		name, err := g.getStringInput()
		if err != nil {
			return err
		}
		g.players = append(g.players, NewHumanPlayer(name, g.scanner))
	}

	// Setup computer players
	for i := 0; i < numComputers; i++ {
		name, strategy, actionTargetStrategy, positiveActionTargetStrategy, err := g.getComputerPlayerSetup(i + 1)
		if err != nil {
			return err
		}
		g.players = append(g.players, NewComputerPlayer(name, strategy, actionTargetStrategy, positiveActionTargetStrategy))
		g.printf("  → Added: %s\n", g.players[len(g.players)-1].GetName())
	}

	if numHumans == 0 {
		g.printf("\n🎮 Starting AI-only Flip 7 with %d computer players!\n", numComputers)
		g.println("🍿 Sit back and watch the AIs battle it out!")

		// Ask for number of games to simulate
		g.printf("\nHow many games would you like to simulate? ")
		numGames, err := g.getIntInput(1, math.MaxInt)
		if err != nil {
			return err
		}

		if numGames > 1 {
			g.SetSilentMode(true)
			return g.runMultipleGames(numGames)
		}
	} else {
		g.printf("\n🎮 Starting Flip 7 with %d humans and %d computers!\n", numHumans, numComputers)
	}
	return nil
}

// getComputerPlayerSetup handles setup for a single computer player
var computerNames = []string{
	"HAL", "Data", "GLaDOS", "WALL-E", "EVE", "R2D2", "C3PO", "T-800",
	"Skynet", "Optimus", "Megatron", "Bender", "WOPR", "Cortana", "Marvin",
}

func (g *Game) getComputerPlayerSetup(computerNum int) (string, HitOrStayStrategy, ActionTargetStrategy, ActionTargetStrategy, error) {
	var name string
	if len(computerNames) > 0 {
		nameIndex := rand.Intn(len(computerNames))
		name = computerNames[nameIndex]
		computerNames = slices.Delete(computerNames, nameIndex, nameIndex+1)
	} else {
		name = fmt.Sprintf("Computer %d", computerNum)
	}

	g.printf("\nComputer Player %d:\n", computerNum)
	g.println("Choose AI strategy:")
	g.println("  1) Plays to 20")
	g.println("  2) Plays to 25")
	g.println("  3) Plays to 30")
	g.println("  4) Plays to 35")
	g.println("  5) Hit p(BUST) < 0.2")
	g.println("  6) Hit p(BUST) < 0.25")
	g.println("  7) Hit p(BUST) < 0.3")
	g.println("  8) Hit p(BUST) < 0.35")
	g.println("  9) Hit p(BUST) < 0.4")
	g.println("  10) FLIP 7")
	g.println("  11) Random")
	g.println("  12) Adaptive Bust Prob (0.3)")
	g.println("  13) Expected Value")
	g.println("  14) Hybrid Strategy")
	g.println("  15) Gap-Based Strategy")
	g.println("  16) Optimal Strategy")
	g.print("Enter choice (1-16): ")

	choice, err := g.getIntInput(1, 16)
	if err != nil {
		choice = 16 // Default to Optimal
	}

	strategy, actionTargetStrategy, positiveActionTargetStrategy := getStrategyByChoice(choice, &name)
	return name, strategy, actionTargetStrategy, positiveActionTargetStrategy, nil
}

func getStrategyByChoice(choice int, name *string) (HitOrStayStrategy, ActionTargetStrategy, ActionTargetStrategy) {
	var strategy HitOrStayStrategy
	var actionTargetStrategy ActionTargetStrategy = TargetLeaderStrategy
	var positiveActionTargetStrategy ActionTargetStrategy = TargetLastPlaceStrategy

	switch choice {
	case 1:
		*name += " (20)"
		strategy = PlayRoundTo(20)
	case 2:
		*name += " (25)"
		strategy = PlayRoundTo(25)
	case 3:
		*name += " (30)"
		strategy = PlayRoundTo(30)
	case 4:
		*name += " (35)"
		strategy = PlayRoundTo(35)
	case 5:
		*name += " (p0.2)"
		strategy = PlayToBustProbability(0.2)
	case 6:
		*name += " (p0.25)"
		strategy = PlayToBustProbability(0.25)
	case 7:
		*name += " (p0.3)"
		strategy = PlayToBustProbability(0.3)
	case 8:
		*name += " (p0.35)"
		strategy = PlayToBustProbability(0.35)
	case 9:
		*name += " (p0.4)"
		strategy = PlayToBustProbability(0.4)
	case 10:
		*name += " (hit)"
		strategy = AlwaysHitStrategy
	case 11:
		*name += " (rand)"
		strategy = RandomHitOrStayStrategy
		actionTargetStrategy = TargetRandomStrategy
		positiveActionTargetStrategy = TargetRandomStrategy
	case 12:
		*name += " (adapt0.3)"
		strategy = AdaptiveBustProbabilityStrategy(0.3)
	case 13:
		*name += " (exp)"
		strategy = ExpectedValueStrategy
	case 14:
		*name += " (hybrid)"
		strategy = HybridStrategy
	case 15:
		*name += " (gap)"
		strategy = GapBasedStrategy
	case 16:
		*name += " (opt)"
		strategy = OptimalStrategy
	default:
		// Default to Optimal Strategy for invalid choices
		*name += " (opt)"
		strategy = OptimalStrategy
	}

	return strategy, actionTargetStrategy, positiveActionTargetStrategy
}

// buildGameState creates a GameState for AI decision making
func (g *Game) buildGameState() *GameState {
	activePlayers := make([]PlayerInterface, 0)
	for _, p := range g.players {
		if p.IsActive() {
			activePlayers = append(activePlayers, p)
		}
	}

	var currentLeader PlayerInterface
	maxScore := -1
	for _, p := range g.players {
		if p.GetTotalScore() > maxScore {
			maxScore = p.GetTotalScore() + p.CalculateRoundScore()
			currentLeader = p
		}
	}

	return &GameState{
		Round:         g.round,
		Players:       g.players,
		ActivePlayers: activePlayers,
		CurrentLeader: currentLeader,
		CardsInDeck:   g.deck.cards,
	}
}

// endRoundForFlip7 marks all players except the Flip 7 achiever as non-active
func (g *Game) endRoundForFlip7(flip7Player PlayerInterface) {
	for _, player := range g.players {
		if player != flip7Player && player.IsActive() {
			player.Stay()
			player.CalculateRoundScore()
		}
	}
}

// runMultipleGames runs multiple AI-only games and tracks statistics
func (g *Game) runMultipleGames(numGames int) error {
	g.printf("\n🎲 Running %d games for statistical analysis...\n", numGames)

	// Track wins for each player
	playerWins := make(map[string]int)
	playerNames := make([]string, len(g.players))

	// Initialize player names and win counters
	for i, player := range g.players {
		playerNames[i] = player.GetName()
		playerWins[player.GetName()] = 0
	}

	// Track time for progress reporting
	startTime := time.Now()
	lastProgressTime := startTime

	// Run the games
	for gameNum := 1; gameNum <= numGames; gameNum++ {
		// Show progress every 5 seconds or for first game
		now := time.Now()
		if gameNum == 1 || now.Sub(lastProgressTime) >= 5*time.Second {
			elapsed := now.Sub(startTime)
			g.printf("⚡ Game %d/%d... (%.1fs elapsed)\n", gameNum, numGames, elapsed.Seconds())
			lastProgressTime = now
		}

		// Reset the game state
		g.resetGameState()

		// Enable silent mode for simulation
		g.SetSilentMode(true)

		// Run a single game using regular methods (now silent)
		err := g.runSingleGame()
		if err != nil {
			return fmt.Errorf("error in game %d: %v", gameNum, err)
		}

		// Track the winner
		winner := g.getWinner()
		playerWins[winner.GetName()]++

		// Disable silent mode to show progress
		g.SetSilentMode(false)
	}

	// Display statistics
	g.displayGameStatistics(numGames, playerWins, playerNames)
	return nil
}

// resetGameState resets the game for a new game
func (g *Game) resetGameState() {
	g.round = 1
	g.dealerIdx = 0

	// Reset all players
	for _, player := range g.players {
		discardedCards := player.ResetForNewRound()
		for _, card := range discardedCards {
			g.deck.DiscardCard(card)
		}
		// Reset total score for new game
		if basePlayer, ok := player.(*ComputerPlayer); ok {
			basePlayer.TotalScore = 0
		}
	}

	// Reset deck
	g.deck = NewDeck()
}

// runSingleGame runs a single game (output controlled by silentMode)
func (g *Game) runSingleGame() error {
	// Main game loop
	for !g.hasWinner() {
		if err := g.playRound(); err != nil {
			return err
		}
		g.nextRound()
	}
	return nil
}

// displayGameStatistics shows the final win-rate statistics
func (g *Game) displayGameStatistics(numGames int, playerWins map[string]int, playerNames []string) {
	g.printf("\n" + strings.Repeat("=", 60) + "\n")
	g.printf("🏆 SIMULATION RESULTS - %d GAMES COMPLETED\n", numGames)
	g.printf(strings.Repeat("=", 60) + "\n")

	// Sort players by win count (descending)
	type playerStat struct {
		name string
		wins int
		rate float64
	}

	var stats []playerStat
	for _, name := range playerNames {
		wins := playerWins[name]
		rate := float64(wins) / float64(numGames) * 100
		stats = append(stats, playerStat{name, wins, rate})
	}

	// Sort by wins (descending)
	slices.SortFunc(stats, func(a, b playerStat) int {
		return b.wins - a.wins
	})

	// Display results
	g.printf("% -20s %8s %10s %12s\n", "PLAYER", "WINS", "WIN RATE", "PERFORMANCE")
	g.printf(strings.Repeat("-", 60) + "\n")

	for i, stat := range stats {
		var medal string
		switch i {
		case 0:
			medal = "🥇"
		case 1:
			medal = "🥈"
		case 2:
			medal = "🥉"
		default:
			medal = "  "
		}

		var performance string
		if stat.rate >= 50 {
			performance = "🔥 DOMINANT"
		} else if stat.rate >= 35 {
			performance = "💪 STRONG"
		} else if stat.rate >= 20 {
			performance = "👍 DECENT"
		} else {
			performance = "😔 WEAK"
		}

		g.printf("% -20s %8d %9.1f%% %12s %s\n",
			stat.name, stat.wins, stat.rate, performance, medal)
	}

	g.printf(strings.Repeat("-", 60) + "\n")
	g.printf("Total Games: %d\n", numGames)

	// Additional statistics
	if len(stats) > 1 {
		winner := stats[0]
		runnerUp := stats[1]
		margin := winner.rate - runnerUp.rate
		g.printf("Victory Margin: %.1f%% (%s vs %s)\n",
			margin, winner.name, runnerUp.name)
	}

	g.printf(strings.Repeat("=", 60) + "\n")
}
