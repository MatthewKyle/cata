package mage

import (
	"time"

	"github.com/wowsims/cata/sim/core"
	"github.com/wowsims/cata/sim/core/stats"
)

//"github.com/wowsims/cata/sim/core/proto"

func (mage *Mage) ApplyFireTalents() {

	// Cooldowns/Special Implementations
	mage.applyIgnite()
	mage.applyImpact()
	mage.applyHotStreak()
	mage.applyMoltenFury()
	mage.applyMasterOfElements()
	mage.applyPyromaniac()

	// Improved Fire Blast
	if mage.Talents.ImprovedFireBlast > 0 {
		mage.AddStaticMod(core.SpellModConfig{
			ClassMask:  MageSpellFireBlast,
			FloatValue: 4 * float64(mage.Talents.ImprovedFireBlast) * core.CritRatingPerCritChance,
			Kind:       core.SpellMod_BonusCrit_Rating,
		})
	}

	// Fire Power
	if mage.Talents.FirePower > 0 {
		mage.AddStaticMod(core.SpellModConfig{
			School:     core.SpellSchoolFire,
			FloatValue: 0.01 * float64(mage.Talents.FirePower),
			Kind:       core.SpellMod_DamageDone_Flat,
		})
	}

	// Improved Scorch
	if mage.Talents.ImprovedScorch > 0 {
		mage.AddStaticMod(core.SpellModConfig{
			ClassMask:  MageSpellScorch,
			FloatValue: -0.5 * float64(mage.Talents.ImprovedScorch),
			Kind:       core.SpellMod_PowerCost_Pct,
		})
	}

	// Improved Flamestrike
	if mage.Talents.ImprovedFlamestrike > 0 {
		mage.AddStaticMod(core.SpellModConfig{
			ClassMask:  MageSpellFlamestrike,
			FloatValue: -0.5 * float64(mage.Talents.ImprovedFlamestrike),
			Kind:       core.SpellMod_CastTime_Pct,
		})
	}

	// Critical Mass
	if mage.Talents.CriticalMass > 0 {
		mage.AddStaticMod(core.SpellModConfig{
			ClassMask:  MageSpellLivingBombDot | MageSpellLivingBombExplosion | MageSpellFlameOrb,
			FloatValue: 0.05 * float64(mage.Talents.CriticalMass),
			Kind:       core.SpellMod_DamageDone_Pct,
		})

		criticalMassDebuff := mage.NewEnemyAuraArray(core.CriticalMassAura)

		core.MakeProcTriggerAura(&mage.Unit, core.ProcTrigger{
			Name:           "Critical Mass Trigger",
			Callback:       core.CallbackOnSpellHitDealt,
			ClassSpellMask: MageSpellPyroblast | MageSpellScorch,
			Outcome:        core.OutcomeLanded,
			ProcChance:     float64(mage.Talents.CriticalMass) / 3.0,
			Handler: func(sim *core.Simulation, spell *core.Spell, result *core.SpellResult) {
				criticalMassDebuff.Get(result.Target).Activate(sim)
			},
		})
	}
}

// Master of Elements
func (mage *Mage) applyMasterOfElements() {
	if mage.Talents.MasterOfElements == 0 {
		return
	}

	refundCoeff := 0.15 * float64(mage.Talents.MasterOfElements)
	manaMetrics := mage.NewManaMetrics(core.ActionID{SpellID: 29077})

	mage.RegisterAura(core.Aura{
		Label:    "Master of Elements",
		Duration: core.NeverExpires,
		OnReset: func(aura *core.Aura, sim *core.Simulation) {
			aura.Activate(sim)
		},
		OnSpellHitDealt: func(aura *core.Aura, sim *core.Simulation, spell *core.Spell, result *core.SpellResult) {
			if spell.ProcMask.Matches(core.ProcMaskMeleeOrRanged) {
				return
			}
			if spell.CurCast.Cost == 0 {
				return
			}
			if result.DidCrit() {
				if refundCoeff < 0 {
					mage.SpendMana(sim, -1*spell.DefaultCast.Cost*refundCoeff, manaMetrics)
				} else {
					mage.AddMana(sim, spell.DefaultCast.Cost*refundCoeff, manaMetrics)
				}
			}
		},
	})
}

func (mage *Mage) applyHotStreak() {
	if !mage.Talents.HotStreak {
		return
	}

	ImprovedHotStreakProcChance := float64(mage.Talents.ImprovedHotStreak) * 0.5
	BaseHotStreakProcChance := float64(-2.7*(mage.GetStat(stats.SpellCrit)/core.CritRatingPerCritChance)/100 + 0.9) // EJ settled on -2.7*critChance+0.9

	// Unimproved Hot Streak Proc Aura
	mage.HotStreakAura = mage.RegisterAura(core.Aura{
		Label:    "Hot Streak",
		ActionID: core.ActionID{SpellID: 48108},
		Duration: time.Second * 10,
	})

	// Improved Hotstreak Crit Stacking Aura
	hotStreakCritAura := mage.RegisterAura(core.Aura{
		Label:     "Hot Streak Proc Aura",
		ActionID:  core.ActionID{SpellID: 44448}, //, Tag: 1}, Removing Tag gets rid of the (??) in Timeline
		MaxStacks: 2,
		Duration:  time.Hour,
	})

	// Aura to allow the character to track crits
	mage.RegisterAura(core.Aura{
		Label:    "Hot Streak Trigger",
		Duration: core.NeverExpires,
		OnReset: func(aura *core.Aura, sim *core.Simulation) {
			aura.Activate(sim)
		},
		OnSpellHitDealt: func(aura *core.Aura, sim *core.Simulation, spell *core.Spell, result *core.SpellResult) {
			if !spell.Flags.Matches(HotStreakSpells) {
				return
			}

			// Pyroblast! cannot trigger hot streak
			// TODO can Pyroblast! *reset* hot streak crit streak? This implementation assumes no.
			// If so, will need to envelope it around the hot streak checks
			if spell.ClassSpellMask == MageSpellPyroblast && spell.CurCast.CastTime == 0 {
				return
			}
			// Hot Streak Base Talent Proc
			if result.DidCrit() {
				if sim.Proc(BaseHotStreakProcChance, "Hot Streak") {
					mage.HotStreakAura.Activate(sim)
				}
			}

			// Improved Hot Streak
			if mage.Talents.ImprovedHotStreak > 0 {
				// If you didn't crit, reset your crit counter
				if !result.DidCrit() {
					hotStreakCritAura.SetStacks(sim, 0)
					hotStreakCritAura.Deactivate(sim)
					return
				}

				// If you did crit, check against talents to see if you proc
				// If you proc and had 1 stack, set crit counter to 0 and give hot streak.
				if hotStreakCritAura.GetStacks() == 1 {
					if sim.Proc(ImprovedHotStreakProcChance, "Improved Hot Streak") {
						hotStreakCritAura.SetStacks(sim, 0)
						hotStreakCritAura.Deactivate(sim)

						mage.HotStreakAura.Activate(sim)
					}

					// If you proc and had 0 stacks of crits, add to your crit counter.
					// No idea if 1 out of 2 talent points means you have a 50% chance to
					// add to the 1st stack of crit, or only the 2nd. Doesn't seem
					// all that important to check since every fire mage in the world
					// will go 2 out of 2 points, but worth researching.
					// If it checks 1st crit as well, can add a proc check to this too
				} else {
					hotStreakCritAura.Activate(sim)
					hotStreakCritAura.AddStack(sim)
				}
			}
		},
	})
}

func (mage *Mage) applyPyromaniac() {
	if mage.Talents.Pyromaniac == 0 {
		return
	}

	pyromaniacMod := mage.AddDynamicMod(core.SpellModConfig{
		ClassMask:  MageSpellsAll,
		FloatValue: -.05 * float64(mage.Talents.Pyromaniac),
		Kind:       core.SpellMod_CastTime_Pct,
	})

	mage.RegisterAura(core.Aura{
		Label:    "Pyromaniac Trackers",
		ActionID: core.ActionID{SpellID: 83582},
		Duration: core.NeverExpires,
		OnReset: func(aura *core.Aura, sim *core.Simulation) {
			if len(sim.AllUnits) < 3 {
				return
			}
			aura.Activate(sim)
		},
		OnCastComplete: func(aura *core.Aura, sim *core.Simulation, spell *core.Spell) {
			dotSpells := []*core.Spell{mage.LivingBomb, mage.Ignite, mage.PyroblastDot, mage.Combustion}
			activeDotTargets := 0
			for _, aoeTarget := range sim.Encounter.TargetUnits {
				for _, spells := range dotSpells {
					if spells.Dot(aoeTarget).IsActive() {
						activeDotTargets++
						break
					}
				}
			}
			if activeDotTargets >= 3 && !pyromaniacMod.IsActive {
				pyromaniacMod.Activate()
			} else if activeDotTargets < 3 && pyromaniacMod.IsActive {
				pyromaniacMod.Deactivate()
			}
		},
	})
}

func (mage *Mage) applyMoltenFury() {
	if mage.Talents.MoltenFury == 0 {
		return
	}

	moltenFuryMod := mage.AddDynamicMod(core.SpellModConfig{
		ClassMask:  MageSpellsAll,
		FloatValue: .04 * float64(mage.Talents.MoltenFury),
		Kind:       core.SpellMod_DamageDone_Pct,
	})

	mage.RegisterResetEffect(func(sim *core.Simulation) {
		sim.RegisterExecutePhaseCallback(func(sim *core.Simulation, isExecute int32) {
			if isExecute == 35 {
				moltenFuryMod.Activate()

				// For some reason Molten Fury doesn't apply to living bomb DoT, so cancel it out.
				// 4/15/24 - Comment above was from before. Worth checking this out.
				/*if mage.LivingBomb != nil {
					mage.LivingBomb.DamageMultiplier /= multiplier
				}*/
			}
		})
	})
}

func (mage *Mage) applyIgnite() {

	if mage.Talents.Ignite == 0 {
		return
	}

	const IgniteTicksFresh = 2
	//const IgniteTicksRefresh = 3

	// Ignite proc listener
	mage.RegisterAura(core.Aura{
		Label:    "Ignite Talent",
		Duration: core.NeverExpires,
		OnReset: func(aura *core.Aura, sim *core.Simulation) {
			aura.Activate(sim)
		},
		OnSpellHitDealt: func(aura *core.Aura, sim *core.Simulation, spell *core.Spell, result *core.SpellResult) {
			if !spell.ProcMask.Matches(core.ProcMaskSpellDamage) {
				return
			}
			// EJ post says combustion crits do not proc ignite
			// https://web.archive.org/web/20120219014159/http://elitistjerks.com/f75/t110187-cataclysm_mage_simulators_formulators/p3/#post1824829
			if spell.SpellSchool.Matches(core.SpellSchoolFire) && result.DidCrit() && spell != mage.Combustion {
				mage.procIgnite(sim, result, mage.Ignite.Dot(mage.CurrentTarget).IsActive())
			}
		},
		OnPeriodicDamageDealt: func(aura *core.Aura, sim *core.Simulation, spell *core.Spell, result *core.SpellResult) {
			if !spell.ProcMask.Matches(core.ProcMaskSpellDamage) {
				return
			}
			if mage.LivingBomb != nil && result.DidCrit() {
				mage.procIgnite(sim, result, mage.Ignite.Dot(mage.CurrentTarget).IsActive())
			}
		},
	})

	actionId := core.ActionID{SpellID: 12846}

	mage.IgniteDamageTracker = core.MakePermanent(mage.RegisterAura(core.Aura{
		ActionID:  actionId,
		Label:     "Ignite Damage Tracker",
		Duration:  core.NeverExpires,
		MaxStacks: 1000000,
		OnReset: func(aura *core.Aura, sim *core.Simulation) {
			aura.Activate(sim)
		},
	}))

	// The ignite dot
	mage.Ignite = mage.RegisterSpell(core.SpellConfig{
		ActionID:       actionId,
		SpellSchool:    core.SpellSchoolFire,
		ProcMask:       core.ProcMaskProc,
		Flags:          core.SpellFlagIgnoreModifiers,
		ClassSpellMask: MageSpellIgnite,

		DamageMultiplier: 1,
		ThreatMultiplier: 1,

		Dot: core.DotConfig{
			Aura: core.Aura{
				Label: "Ignite",
				Tag:   "IgniteDot",
			},
			NumberOfTicks: IgniteTicksFresh,
			TickLength:    time.Second * 2,
			OnSnapshot: func(sim *core.Simulation, target *core.Unit, dot *core.Dot, isRollover bool) {

			},
			OnTick: func(sim *core.Simulation, target *core.Unit, dot *core.Dot) {
				// Need to do mastery here
				currentMastery := 1.22 + 0.028*mage.GetMasteryPoints()

				result := dot.Spell.CalcPeriodicDamage(sim, target, dot.SnapshotBaseDamage*currentMastery, dot.OutcomeTick)
				dot.Spell.DealPeriodicDamage(sim, result)
			},
		},

		ApplyEffects: func(sim *core.Simulation, target *core.Unit, spell *core.Spell) {
			spell.SpellMetrics[target.UnitIndex].Hits++
			spell.Dot(target).ApplyOrReset(sim)
		},
	})
}

func (mage *Mage) procIgnite(sim *core.Simulation, result *core.SpellResult, isActive bool) {
	const IgniteTicksFresh = 2
	//const IgniteTicksRefresh = 3
	igniteDamageMultiplier := []float64{0.0, 0.13, 0.26, 0.40}[mage.Talents.Ignite]

	dot := mage.Ignite.Dot(result.Target)

	newDamage := result.Damage * igniteDamageMultiplier

	// if ignite was still active, we store up the remaining damage to be added to the next application
	outstandingDamage := core.TernaryFloat64(dot.IsActive(), dot.SnapshotBaseDamage*float64(dot.NumberOfTicks-dot.TickCount), 0)
	dot.SnapshotAttackerMultiplier = 1

	// OG CATA VERSION
	// 1st ignite application = 4s, split into 2 ticks (2s, 0s)
	// Ignite refreshes: Duration = 4s + MODULO(remaining duration, 2), max 6s. Split over 3 ticks at 4s, 2s, 0s.
	// Do not refresh ignites if there is more than 4s left on duration.
	/*
		if isActive {
			if mage.Ignite.Dot(result.Target).RemainingDuration(sim) > time.Millisecond*4000 {
				return
			}
			mage.Ignite.Dot(result.Target).NumberOfTicks = IgniteTicksRefresh
			dot.SnapshotBaseDamage = ((outstandingDamage + newDamage) / float64(IgniteTicksRefresh))
			mage.Ignite.Cast(sim, result.Target)
		} else {
			mage.Ignite.Dot(result.Target).NumberOfTicks = IgniteTicksFresh
			dot.SnapshotBaseDamage = ((outstandingDamage + newDamage) / float64(IgniteTicksFresh))
			mage.Ignite.Cast(sim, result.Target)
		}
	*/

	// THIS IS THE VERSION CURRENTLY IN BETA aka old ignite
	// Add the remaining damage to the new ignite proc, divide it over 2 ticks
	dot.SnapshotBaseDamage = ((outstandingDamage + newDamage) / float64(IgniteTicksFresh))
	mage.Ignite.Cast(sim, result.Target)
	if mage.IgniteDamageTracker.IsActive() {
		mage.IgniteDamageTracker.SetStacks(sim, int32(dot.SnapshotBaseDamage))
	}
}

func (mage *Mage) applyImpact() {

	if mage.Talents.Impact == 0 {
		return
	}

	// TODO make this work :)
	// Currently casts a fresh set of DoTs
	// afaik it should spread exact copies of the DoTs
	impactAura := mage.RegisterAura(core.Aura{
		Label:    "Impact",
		ActionID: core.ActionID{SpellID: 64343},
		Duration: time.Second * 10,
		OnCastComplete: func(aura *core.Aura, sim *core.Simulation, spell *core.Spell) {

			if spell.ClassSpellMask == MageSpellFireBlast {
				originalTarget := mage.CurrentTarget

				duplicatableDots := map[*core.Spell]float64{
					mage.LivingBombImpact:   mage.LivingBomb.Dot(originalTarget).SnapshotBaseDamage,
					mage.PyroblastDotImpact: mage.PyroblastDot.Dot(originalTarget).SnapshotBaseDamage,
					mage.Ignite:             mage.Ignite.Dot(originalTarget).SnapshotBaseDamage,
					mage.Combustion:         mage.Combustion.Dot(originalTarget).SnapshotBaseDamage,
				}
				for _, aoeTarget := range sim.Encounter.TargetUnits {
					if aoeTarget == originalTarget {
						continue
					}
					for spell, damage := range duplicatableDots {
						spell.Dot(aoeTarget).Snapshot(aoeTarget, damage)
						spell.Dot(aoeTarget).Apply(sim)
					}
				}
				aura.Deactivate(sim)
			}
		},
	})

	core.MakeProcTriggerAura(&mage.Unit, core.ProcTrigger{
		Name:           "Impact Trigger",
		Callback:       core.CallbackOnSpellHitDealt,
		ClassSpellMask: MageSpellsAll,
		ProcChance:     0.05 * float64(mage.Talents.Impact),
		Handler: func(sim *core.Simulation, spell *core.Spell, result *core.SpellResult) {
			mage.FireBlast.CD.Reset()
			impactAura.Activate(sim)
		},
	})
}
