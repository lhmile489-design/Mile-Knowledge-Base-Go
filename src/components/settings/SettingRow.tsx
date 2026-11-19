import type { ReactNode } from "react";

/** One titled group of setting rows (port of the legacy SettingRow.tsx). */
export function SettingGroup({
  title,
  description,
  footnote,
  testid,
  children,
}: {
  readonly title: string;
  readonly description?: string;
  readonly footnote?: string;
  readonly testid?: string;
  readonly children: ReactNode;
}) {
  return (
    <section className="setting-group" data-testid={testid} aria-label={title}>
      <div className="setting-group-head">
        <h2 className="setting-group-title">{title}</h2>
        {description ? <span className="setting-group-desc">{description}</span> : null}
      </div>
      <div className="setting-card">{children}</div>
      {footnote ? <p className="field-hint">{footnote}</p> : null}
    </section>
  );
}

/** One row: label + help on the left, the control on the right (or stacked). */
export function SettingRow({
  label,
  help,
  helpId,
  stacked,
  control,
  testid,
  children,
}: {
  readonly label: string;
  readonly help?: string;
  readonly helpId?: string;
  readonly stacked?: boolean;
  readonly control?: ReactNode;
  readonly testid?: string;
  readonly children?: ReactNode;
}) {
  return (
    <div className="setting-row" data-stacked={stacked ? "true" : undefined} data-testid={testid}>
      <div className="setting-row-label">
        {label}
        {help ? (
          <div className="setting-row-help" id={helpId}>
            {help}
          </div>
        ) : null}
      </div>
      {stacked ? children : <div className="setting-row-control">{control}</div>}
    </div>
  );
}
