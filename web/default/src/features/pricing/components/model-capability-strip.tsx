/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  Brain,
  CornerDownRight,
  Image as ImageIcon,
  Layers,
  ScanEye,
  Target,
  Video,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  type ConfigurableInputModality,
  getConfiguredInputModalities,
  getContextMeterRatio,
  getInheritedCapabilityTarget,
  getInheritedPurposeTarget,
  getOrderedPurposes,
  getPurposeLabels,
  getThinkingLevels,
} from '../lib/model-capabilities'
import { formatTokenCount } from '../lib/model-metadata'
import type { PricingModel } from '../types'

const MODALITY_META: Record<
  ConfigurableInputModality,
  { icon: LucideIcon; labelKey: string }
> = {
  image: { icon: ImageIcon, labelKey: 'Image' },
  video: { icon: Video, labelKey: 'Video' },
}

const ACCENT_CLASS = {
  context: 'text-sky-600 dark:text-sky-400',
  multimodal: 'text-emerald-600 dark:text-emerald-400',
  thinking: 'text-violet-600 dark:text-violet-400',
  purposes: 'text-amber-600 dark:text-amber-400',
} as const

const CONTEXT_METER_SEGMENTS = 8
const CONTEXT_METER_FILL_CLASS = 'bg-sky-500 dark:bg-sky-400'

function StatLabel(props: {
  icon: LucideIcon
  label: string
  configured: boolean
  accent: string
  className?: string
}) {
  const Icon = props.icon
  return (
    <span
      className={cn(
        'text-muted-foreground/70 inline-flex min-w-0 items-center gap-1 text-[10px] leading-4 font-medium tracking-wider uppercase',
        props.className
      )}
    >
      <Icon
        aria-hidden='true'
        className={cn(
          'size-3 shrink-0',
          props.configured ? props.accent : 'text-muted-foreground/40'
        )}
      />
      <span className='truncate'>{props.label}</span>
    </span>
  )
}

/** Placeholder for a value the operator has not configured. */
function UnsetValue(props: { title: string }) {
  return (
    <span
      title={props.title}
      className='text-muted-foreground/40 font-mono text-sm leading-[18px] select-none'
    >
      —
    </span>
  )
}

function StatCell(props: {
  icon: LucideIcon
  label: string
  configured: boolean
  accent: string
  children: React.ReactNode
}) {
  return (
    // Top-aligned so the labels of both cells stay on one line when the
    // modality chips wrap onto a second row.
    <div className='flex min-w-0 flex-col gap-0.5 px-2.5 py-2 sm:px-3'>
      <StatLabel
        icon={props.icon}
        label={props.label}
        configured={props.configured}
        accent={props.accent}
      />
      {/* A flex container keeps numbers, chips and the "—" placeholder on one
          box: as inline text the monospace dash would grow the line box. */}
      <div className='text-foreground flex min-w-0 items-center text-sm leading-[18px] font-semibold tabular-nums'>
        {props.children}
      </div>
    </div>
  )
}

/** Context size plus a segmented meter (8K → 1M log scale). */
function ContextValue(props: { tokens: number }) {
  const { t } = useTranslation()
  const ratio = getContextMeterRatio(props.tokens)
  const litSegments = Math.max(1, Math.ceil(ratio * CONTEXT_METER_SEGMENTS))

  return (
    <div
      className='flex w-full items-center gap-2'
      title={`${props.tokens.toLocaleString()} ${t('tokens')}`}
    >
      <span className='truncate font-mono'>
        {formatTokenCount(props.tokens)}
      </span>
      <span
        aria-hidden='true'
        className='bg-muted-foreground/10 ml-auto flex h-1.5 shrink-0 items-center gap-0.5 rounded-sm p-0.5'
      >
        {Array.from({ length: CONTEXT_METER_SEGMENTS }, (_, index) => (
          <span
            key={index}
            className={cn(
              'h-1 w-[3px] rounded-[1px]',
              index < litSegments
                ? CONTEXT_METER_FILL_CLASS
                : 'bg-muted-foreground/15'
            )}
          />
        ))}
      </span>
    </div>
  )
}

function ModalityChip(props: { modality: ConfigurableInputModality }) {
  const { t } = useTranslation()
  const Icon = MODALITY_META[props.modality].icon
  return (
    <span className='bg-background text-foreground/80 inline-flex h-[18px] items-center gap-1 rounded-md border px-1.5 text-[11px] leading-4 font-medium'>
      <Icon aria-hidden='true' className='size-3 shrink-0' />
      {t(MODALITY_META[props.modality].labelKey)}
    </span>
  )
}

function LevelChip(props: { level: string }) {
  return (
    <span className='bg-background text-foreground/80 inline-flex h-[18px] items-center rounded-md border px-1.5 font-mono text-[10px] leading-4 font-medium'>
      {props.level}
    </span>
  )
}

function PurposeChip(props: { label: string }) {
  return (
    <span className='bg-background text-foreground/80 inline-flex h-[18px] items-center rounded-md border px-1.5 text-[11px] leading-4 font-medium'>
      {props.label}
    </span>
  )
}

type ChipRow = {
  key: string
  icon: LucideIcon
  label: string
  configured: boolean
  accent: string
  chips: React.ReactNode
}

// Width of the "思考级别" label (12px icon + 4px gap + four CJK characters), so
// that a shorter label such as "用途" starts its chips at the same x. Longer,
// translated labels simply grow instead of squeezing their chips.
const CHIP_LABEL_MIN_WIDTH_CLASS = 'min-w-[58px] shrink-0'

/**
 * Stacked rows of chips, each as "label: chips". Labels only share a minimum
 * width: with one common column a long translated label (e.g. the English
 * "Thinking level") would leave the chips of every row just a sliver of room
 * on narrow cards.
 */
function ChipRows(props: { rows: ChipRow[]; unsetTitle: string }) {
  return (
    <>
      {props.rows.map((row) => (
        <div
          key={row.key}
          className='flex items-center gap-2 px-2.5 py-1.5 sm:px-3'
        >
          <StatLabel
            icon={row.icon}
            label={row.label}
            configured={row.configured}
            accent={row.accent}
            className={CHIP_LABEL_MIN_WIDTH_CLASS}
          />
          {row.configured ? (
            <div className='flex min-w-0 flex-1 flex-wrap items-center gap-1'>
              {row.chips}
            </div>
          ) : (
            <UnsetValue title={props.unsetTitle} />
          )}
        </div>
      ))}
    </>
  )
}

/**
 * Note shown wherever parts of a model's info are inherited from its billing
 * follow target, so readers know those values are not its own configuration.
 */
export function ModelFollowNote(props: {
  target: string
  /** The capabilities (context, modalities, thinking levels) are inherited */
  capabilities?: boolean
  /** The purposes are inherited */
  purposes?: boolean
  className?: string
}) {
  const { t } = useTranslation()
  let label: string
  if (props.capabilities && props.purposes) {
    label = t('Capabilities and purposes follow')
  } else if (props.capabilities) {
    label = t('Capabilities follow')
  } else {
    label = t('Purposes follow')
  }

  return (
    // Wrapping (instead of truncating) keeps the follow target readable when
    // the translated label is long and the card is narrow.
    <div
      className={cn(
        'text-muted-foreground flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5 text-[11px] leading-4',
        props.className
      )}
    >
      <CornerDownRight
        aria-hidden='true'
        className='size-3 shrink-0 text-sky-600 dark:text-sky-400'
      />
      <span className='shrink-0'>{label}</span>
      <span className='min-w-0 font-mono break-all'>{props.target}</span>
    </div>
  )
}

// Tinted like the card's "follow" badge so the note reads apart from the data rows
const FOLLOW_NOTE_ROW_CLASS =
  'bg-sky-500/[0.06] dark:bg-sky-400/[0.08] px-2.5 py-1.5 sm:px-3'

/**
 * Summary shown on a model card: context size, multimodal inputs, selectable
 * thinking levels and purposes. Every value comes from the model metadata
 * (or its billing follow target when the backend marks it as inherited);
 * unconfigured values are shown as "—" instead of being guessed.
 */
export function ModelCapabilityStrip(props: {
  model: PricingModel
  className?: string
}) {
  const { t } = useTranslation()
  const contextLength = props.model.context_length ?? 0
  const inputModalities = getConfiguredInputModalities(props.model)
  const thinkingLevels = getThinkingLevels(props.model)
  const purposes = getOrderedPurposes(props.model.purposes)
  const purposeLabels = getPurposeLabels(t)
  const unsetTitle = t('Not Set')
  const capabilityTarget = getInheritedCapabilityTarget(props.model)
  const purposeTarget = getInheritedPurposeTarget(props.model)
  const followTarget = capabilityTarget ?? purposeTarget

  return (
    <div
      className={cn(
        'bg-muted/20 divide-y overflow-hidden rounded-lg border',
        props.className
      )}
    >
      <div className='grid grid-cols-2 divide-x'>
        <StatCell
          icon={Layers}
          label={t('Context')}
          configured={contextLength > 0}
          accent={ACCENT_CLASS.context}
        >
          {contextLength > 0 ? (
            <ContextValue tokens={contextLength} />
          ) : (
            <UnsetValue title={unsetTitle} />
          )}
        </StatCell>
        <StatCell
          icon={ScanEye}
          label={t('Multimodal')}
          configured={inputModalities.length > 0}
          accent={ACCENT_CLASS.multimodal}
        >
          {inputModalities.length > 0 ? (
            <div className='flex min-w-0 flex-wrap items-center gap-1'>
              {inputModalities.map((modality) => (
                <ModalityChip key={modality} modality={modality} />
              ))}
            </div>
          ) : (
            <UnsetValue title={unsetTitle} />
          )}
        </StatCell>
      </div>

      <ChipRows
        unsetTitle={unsetTitle}
        rows={[
          {
            key: 'thinking',
            icon: Brain,
            label: t('Thinking level'),
            configured: thinkingLevels.length > 0,
            accent: ACCENT_CLASS.thinking,
            chips: thinkingLevels.map((level) => (
              <LevelChip key={level} level={level} />
            )),
          },
          {
            key: 'purposes',
            icon: Target,
            label: t('Purposes'),
            configured: purposes.length > 0,
            accent: ACCENT_CLASS.purposes,
            chips: purposes.map((purpose) => (
              <PurposeChip
                key={purpose}
                label={purposeLabels[purpose] ?? purpose}
              />
            )),
          },
        ]}
      />

      {followTarget && (
        <ModelFollowNote
          target={followTarget}
          capabilities={Boolean(capabilityTarget)}
          purposes={Boolean(purposeTarget)}
          className={FOLLOW_NOTE_ROW_CLASS}
        />
      )}
    </div>
  )
}
