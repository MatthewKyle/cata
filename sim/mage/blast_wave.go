package mage

import (
	"time"

	"github.com/wowsims/cata/sim/core"
)

func (mage *Mage) registerBlastWaveSpell() {
	/* 	if !mage.Talents.BlastWave {
		return
	} */

	mage.BlastWave = mage.RegisterSpell(core.SpellConfig{
		ActionID:       core.ActionID{SpellID: 11113},
		SpellSchool:    core.SpellSchoolFire,
		ProcMask:       core.ProcMaskSpellDamage,
		Flags:          SpellFlagMage | core.SpellFlagAPL,
		ClassSpellMask: MageSpellBlastWave,
		ManaCost: core.ManaCostOptions{
			BaseCost: 0.07,
		},
		Cast: core.CastConfig{
			DefaultCast: core.Cast{
				GCD: core.GCDDefault,
			},
			CD: core.Cooldown{
				Timer:    mage.NewTimer(),
				Duration: time.Second * 30,
			},
		},
		DamageMultiplierAdditive: 1,
		CritMultiplier:           mage.DefaultSpellCritMultiplier(),
		BonusCoefficient:         0.193,
		ThreatMultiplier:         1,
		ApplyEffects: func(sim *core.Simulation, target *core.Unit, spell *core.Spell) {
			var targetCount int32
			for _, aoeTarget := range sim.Encounter.TargetUnits {
				targetCount++
				baseDamage := sim.Roll(1047, 1233)
				baseDamage *= sim.Encounter.AOECapMultiplier()
				spell.CalcAndDealDamage(sim, aoeTarget, baseDamage, spell.OutcomeMagicHitAndCrit)
			}
			if targetCount > 1 {
				mage.Flamestrike.SkipCastAndApplyEffects(sim, target)
			}
		},
	})
}
