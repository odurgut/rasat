import type { ReactNode } from "react";

type WorkspaceProps = {
  list?: ReactNode;
  work: ReactNode;
  detail?: ReactNode;
};

export function Workspace({ list, work, detail }: WorkspaceProps): ReactNode {
  const cls = ["workspace"];
  if (list) {
    cls.push("has-list");
  }
  if (detail) {
    cls.push("has-detail");
  }
  return (
    <div className={cls.join(" ")}>
      {list ? <aside className="pane-list">{list}</aside> : null}
      <main className="pane-work">{work}</main>
      {detail ? <aside className="pane-detail">{detail}</aside> : null}
    </div>
  );
}
