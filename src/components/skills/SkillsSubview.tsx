import { Fragment, type ReactNode } from "react";

/** Breadcrumb trail shared by the subview shell and the page views. */
export function SkillsCrumbs({ crumbs }: { readonly crumbs: string[] }) {
  return (
    <nav className="skills-crumbs" aria-label="位置">
      {crumbs.map((crumb, index) => {
        const current = index === crumbs.length - 1;
        return (
          <Fragment key={`${crumb}-${index}`}>
            {index > 0 ? (
              <span className="skills-crumb-sep" aria-hidden>
                /
              </span>
            ) : null}
            <span className={current ? "skills-crumb is-current" : "skills-crumb"}>{crumb}</span>
          </Fragment>
        );
      })}
    </nav>
  );
}

/** Full-page subview shell for the skills page (M8 T06): breadcrumb + back,
 * scrolling body and a fixed action bar. The callers keep their own Esc
 * handling and submission logic; this only replaces the floating card. */
export function SkillsSubview({
  testId,
  crumbs,
  backLabel = "返回",
  backDisabled = false,
  onBack,
  footer,
  children,
}: {
  readonly testId: string;
  readonly crumbs: string[];
  readonly backLabel?: string;
  readonly backDisabled?: boolean;
  readonly onBack: () => void;
  readonly footer: ReactNode;
  readonly children: ReactNode;
}) {
  return (
    <section className="skills-subview" data-testid={testId}>
      <header className="skills-subview-head">
        <button type="button" className="skills-subview-back" aria-label={backLabel} onClick={onBack} disabled={backDisabled}>
          ←
        </button>
        <SkillsCrumbs crumbs={crumbs} />
      </header>
      <div className="skills-subview-body">{children}</div>
      <footer className="skills-subview-foot">{footer}</footer>
    </section>
  );
}
