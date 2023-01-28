import { IndividualSimUI } from '../individual_sim_ui';
import { SimUI } from '../sim_ui';
import {
	PseudoStat,
	Spec,
	Stat
} from '../proto/common';
import { IndividualSimSettings } from '../proto/ui';
import { classNames } from '../proto_utils/names';
import { UnitStat } from '../proto_utils/stats';
import { specNames } from '../proto_utils/utils';
import { downloadString } from '../utils';
import { BaseModal } from './base_modal';

export abstract class Exporter extends BaseModal {
	private readonly textElem: HTMLElement;

	constructor(parent: HTMLElement, simUI: SimUI, title: string, allowDownload: boolean) {
		super(parent, 'exporter', {footer: true});

		this.header!.insertAdjacentHTML('afterbegin', `<h5 class="modal-title">${title}</h5>`);
		this.body.innerHTML = `
			<textarea class="exporter-textarea form-control"></textarea>
		`;
		this.footer!.innerHTML = `
			<button class="exporter-button btn btn-primary clipboard-button me-2">
				<i class="fas fa-clipboard"></i>
				Copy to Clipboard
			</button>
			${allowDownload ? `
				<button class="exporter-button btn btn-primary download-button">
					<i class="fa fa-download"></i>
					Download
				</button>
			` : ''
			}
		`;

		this.textElem = this.rootElem.getElementsByClassName('exporter-textarea')[0] as HTMLElement;

		const clipboardButton = this.rootElem.getElementsByClassName('clipboard-button')[0] as HTMLElement;
		clipboardButton.addEventListener('click', event => {
			const data = this.textElem.textContent!;
			if (navigator.clipboard == undefined) {
				alert(data);
			} else {
				navigator.clipboard.writeText(data);
				const originalContent = clipboardButton.innerHTML;
				clipboardButton.style.width = `${clipboardButton.getBoundingClientRect().width.toFixed(3)}px`;
				clipboardButton.innerHTML = `<i class="fas fa-check"></i>&nbsp;Copied`;
				setTimeout(() => {
					clipboardButton.innerHTML = originalContent;
				}, 1500);
			}
		});

		if (allowDownload) {
			const downloadButton = this.rootElem.getElementsByClassName('download-button')[0] as HTMLElement;
			downloadButton.addEventListener('click', event => {
				const data = this.textElem.textContent!;
				downloadString(data, 'wowsims.json');
			});
		}
	}

	protected init() {
		this.textElem.textContent = this.getData();
	}

	abstract getData(): string;
}

export class IndividualLinkExporter<SpecType extends Spec> extends Exporter {
	private readonly simUI: IndividualSimUI<SpecType>;

	constructor(parent: HTMLElement, simUI: IndividualSimUI<SpecType>) {
		super(parent, simUI, 'Sharable Link', false);
		this.simUI = simUI;
		this.init();
	}

	getData(): string {
		return this.simUI.toLink();
	}
}

export class IndividualJsonExporter<SpecType extends Spec> extends Exporter {
	private readonly simUI: IndividualSimUI<SpecType>;

	constructor(parent: HTMLElement, simUI: IndividualSimUI<SpecType>) {
		super(parent, simUI, 'JSON Export', true);
		this.simUI = simUI;
		this.init();
	}

	getData(): string {
		return JSON.stringify(IndividualSimSettings.toJson(this.simUI.toProto()), null, 2);
	}
}

export class Individual80UEPExporter<SpecType extends Spec> extends Exporter {
	private readonly simUI: IndividualSimUI<SpecType>;

	constructor(parent: HTMLElement, simUI: IndividualSimUI<SpecType>) {
		super(parent, simUI, '80Upgrades EP Export', true);
		this.simUI = simUI;
		this.init();
	}

	getData(): string {
		const player = this.simUI.player;
		const epValues = player.getEpWeights();
		const allUnitStats = UnitStat.getAll();

		const namesToWeights: Record<string, number> = {};
		allUnitStats
		.forEach(stat => {
			const statName = Individual80UEPExporter.getName(stat);
			const weight = epValues.getUnitStat(stat);
			if (weight == 0 || statName == '') {
				return;
			}

			// Need to add together stats with the same name (e.g. hit/crit/haste).
			if (namesToWeights[statName]) {
				namesToWeights[statName] += weight;
			} else {
				namesToWeights[statName] = weight;
			}
		});

		return `https://eightyupgrades.com/ep/import?name=${encodeURIComponent(`${specNames[player.spec]} WoWSims Weights`)}` +
			Object.keys(namesToWeights)
				.map(statName => `&${statName}=${namesToWeights[statName].toFixed(3)}`).join('');
	}

	static getName(stat: UnitStat): string {
		if (stat.isStat()) {
			return Individual80UEPExporter.statNames[stat.getStat()];
		} else {
			return Individual80UEPExporter.pseudoStatNames[stat.getPseudoStat()] || '';
		}
	}

	static statNames: Record<Stat, string> = {
		[Stat.StatStrength]: 'strength',
		[Stat.StatAgility]: 'agility',
		[Stat.StatStamina]: 'stamina',
		[Stat.StatIntellect]: 'intellect',
		[Stat.StatSpirit]: 'spirit',
		[Stat.StatSpellPower]: 'spellDamage',
		[Stat.StatMP5]: 'mp5',
		[Stat.StatSpellHit]: 'hitRating',
		[Stat.StatSpellCrit]: 'critRating',
		[Stat.StatSpellHaste]: 'hasteRating',
		[Stat.StatSpellPenetration]: 'spellPen',
		[Stat.StatAttackPower]: 'attackPower',
		[Stat.StatMeleeHit]: 'hitRating',
		[Stat.StatMeleeCrit]: 'critRating',
		[Stat.StatMeleeHaste]: 'hasteRating',
		[Stat.StatArmorPenetration]: 'armorPenRating',
		[Stat.StatExpertise]: 'expertiseRating',
		[Stat.StatMana]: 'mana',
		[Stat.StatEnergy]: 'energy',
		[Stat.StatRage]: 'rage',
		[Stat.StatArmor]: 'armor',
		[Stat.StatRangedAttackPower]: 'attackPower',
		[Stat.StatDefense]: 'defenseRating',
		[Stat.StatBlock]: 'blockRating',
		[Stat.StatBlockValue]: 'blockValue',
		[Stat.StatDodge]: 'dodgeRating',
		[Stat.StatParry]: 'parryRating',
		[Stat.StatResilience]: 'resilienceRating',
		[Stat.StatHealth]: 'health',
		[Stat.StatArcaneResistance]: 'arcaneResistance',
		[Stat.StatFireResistance]: 'fireResistance',
		[Stat.StatFrostResistance]: 'frostResistance',
		[Stat.StatNatureResistance]: 'natureResistance',
		[Stat.StatShadowResistance]: 'shadowResistance',
		[Stat.StatBonusArmor]: 'armorBonus',
	}
	static pseudoStatNames: Partial<Record<PseudoStat, string>> = {
		[PseudoStat.PseudoStatMainHandDps]: 'dps',
		[PseudoStat.PseudoStatRangedDps]: 'rangedDps',
	}
}

export class IndividualPawnEPExporter<SpecType extends Spec> extends Exporter {
	private readonly simUI: IndividualSimUI<SpecType>;

	constructor(parent: HTMLElement, simUI: IndividualSimUI<SpecType>) {
		super(parent, simUI, 'Pawn EP Export', true);
		this.simUI = simUI;
		this.init();
	}

	getData(): string {
		const player = this.simUI.player;
		const epValues = player.getEpWeights();
		const allUnitStats = UnitStat.getAll();

		const namesToWeights: Record<string, number> = {};
		allUnitStats
		.forEach(stat => {
			const statName = IndividualPawnEPExporter.getName(stat);
			const weight = epValues.getUnitStat(stat);
			if (weight == 0 || statName == '') {
				return;
			}

			// Need to add together stats with the same name (e.g. hit/crit/haste).
			if (namesToWeights[statName]) {
				namesToWeights[statName] += weight;
			} else {
				namesToWeights[statName] = weight;
			}
		});

		return `( Pawn: v1: "${specNames[player.spec]} WoWSims Weights": Class=${classNames[player.getClass()]},` +
			Object.keys(namesToWeights)
				.map(statName => `${statName}=${namesToWeights[statName].toFixed(3)}`).join(',') +
			' )';
	}

	static getName(stat: UnitStat): string {
		if (stat.isStat()) {
			return IndividualPawnEPExporter.statNames[stat.getStat()];
		} else {
			return IndividualPawnEPExporter.pseudoStatNames[stat.getPseudoStat()] || '';
		}
	}

	static statNames: Record<Stat, string> = {
		[Stat.StatStrength]: 'Strength',
		[Stat.StatAgility]: 'Agility',
		[Stat.StatStamina]: 'Stamina',
		[Stat.StatIntellect]: 'Intellect',
		[Stat.StatSpirit]: 'Spirit',
		[Stat.StatSpellPower]: 'SpellDamage',
		[Stat.StatMP5]: 'Mp5',
		[Stat.StatSpellHit]: 'HitRating',
		[Stat.StatSpellCrit]: 'CritRating',
		[Stat.StatSpellHaste]: 'HasteRating',
		[Stat.StatSpellPenetration]: 'SpellPen',
		[Stat.StatAttackPower]: 'Ap',
		[Stat.StatMeleeHit]: 'HitRating',
		[Stat.StatMeleeCrit]: 'CritRating',
		[Stat.StatMeleeHaste]: 'HasteRating',
		[Stat.StatArmorPenetration]: 'ArmorPenetration',
		[Stat.StatExpertise]: 'ExpertiseRating',
		[Stat.StatMana]: 'Mana',
		[Stat.StatEnergy]: 'Energy',
		[Stat.StatRage]: 'Rage',
		[Stat.StatArmor]: 'Armor',
		[Stat.StatRangedAttackPower]: 'Ap',
		[Stat.StatDefense]: 'DefenseRating',
		[Stat.StatBlock]: 'BlockRating',
		[Stat.StatBlockValue]: 'BlockValue',
		[Stat.StatDodge]: 'DodgeRating',
		[Stat.StatParry]: 'ParryRating',
		[Stat.StatResilience]: 'ResilienceRating',
		[Stat.StatHealth]: 'Health',
		[Stat.StatArcaneResistance]: 'ArcaneResistance',
		[Stat.StatFireResistance]: 'FireResistance',
		[Stat.StatFrostResistance]: 'FrostResistance',
		[Stat.StatNatureResistance]: 'NatureResistance',
		[Stat.StatShadowResistance]: 'ShadowResistance',
		[Stat.StatBonusArmor]: 'Armor2',
	}
	static pseudoStatNames: Partial<Record<PseudoStat, string>> = {
		[PseudoStat.PseudoStatMainHandDps]: 'MeleeDps',
		[PseudoStat.PseudoStatRangedDps]: 'RangedDps',
	}
}
