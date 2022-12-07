package client

import (
	"time"

	"github.com/zyedidia/generic/queue"
	"github.com/unitoftime/ecs"

	"github.com/unitoftime/mmo"
	"github.com/unitoftime/mmo/serdes"
)

type LastUpdate struct {
	Time time.Time
}

func ClientPollNetworkSystem(networkChannel chan serdes.WorldUpdate,
	updateQueue *queue.Queue[serdes.WorldUpdate]) ecs.System {

	// Read everything from the channel and push it into the updateQueue
	sys := ecs.System{"PollNetwork", func(dt time.Duration) {
	MainLoop:
		for {
			select {
			case update := <-networkChannel:
				updateQueue.Enqueue(update)

			default:
				break MainLoop
			}
		}
	}}

	return sys
}

func ClientPullFromUpdateQueue(world *ecs.World, updateQueue *queue.Queue[serdes.WorldUpdate], playerData *PlayerData) ecs.System {
	// TODO! - dynamic based on connection
	targetQueueSize := mmo.ClientDefaultUpdateQueueSize

	var everyOther int

	// Read a single element from the update queue
	sys := ecs.System{"PullUpdateQueue", func(dt time.Duration) {
		if targetQueueSize > 0 {
			//TODO! - IMPORTANT If I pull out like tick 100, then next tick 102, I know that those should be (2 * 64ms) apart and not 64 ms apart. I somehow need to fix that problem for when dropped packets are recv'ed. Or I need to split the difference and enqueue another thing
			everyOther = (everyOther + 1) % mmo.NetworkTickDivider
			if everyOther != 0 {
				return // skip
			}

			// // TODO - keep track of size
			queueSize := 0
			updateQueue.Each(func(u serdes.WorldUpdate) {
				queueSize++
			})
			if queueSize > targetQueueSize {
				// log.Print("UpdateQueue Desynchronization (TooBig): ", queueSize, targetQueueSize)
				// We want the next tick to run a little bit faster.
				// TODO - Optimization Note: The bigger you make the addition, the faster it gets back to target length, but the more dramatic the entity needs to be sped up to interp that distance. I could also change the %4 to %8 to make the speedup even more minimal. Right now this doesn't seem to noticeable
				everyOther = (everyOther + 1) % mmo.NetworkTickDivider
			}//  else if queueSize < targetQueueSize {
			// 	log.Print("UpdateQueue Desynchronization (TooSmall): ", queueSize, targetQueueSize)
			// 	everyOther = (everyOther + 3) % mmo.NetworkTickDivider // Go back one and rerun
			// 	return
			// }
			// log.Print("UpdateQueueSize: ", queueSize)
		}

		if updateQueue.Empty() {
			return
		}

		update := updateQueue.Dequeue()

		// Update our playerData tick information
		playerData.SetTicks(update.Tick, update.PlayerTick)

		for id, compList := range update.WorldData {
			compList = append(compList, ecs.C(LastUpdate{time.Now()}))
			ecs.Write(world, id, compList...)
		}

		// Delete all the entities in the deleteList
		if update.Delete != nil {
			for _, id := range update.Delete {
				ecs.Delete(world, id)
			}
		}
	}}

	return sys
}
