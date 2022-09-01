import { useCallback, useState } from 'react';
import { Button } from '../components/button';

export const SideBar: React.FC = () => {
    const [sideBarState, hideSideBar] = useState<boolean>(true);

    const sideBarStateCallback = useCallback(() => {
        hideSideBar(!sideBarState);
    }, [sideBarState]);

    return (
        <aside className={`mdc-drawer mdc-drawer--dismissible demo-drawer hidden-` + sideBarState} id={'SideBar'}>
            <nav className="mdc-drawer__drawer sidebar-content">
                <div className="sidebar-header">
                    <Button className="mdc-top-button" icon={'navigate_next'} clickFunction={sideBarStateCallback} />
                    <h1 className="sidebar-header-title">Planned Actions</h1>
                </div>
                <nav className="mdc-drawer__content">
                    <div id="icon-with-text-demo" className="mdc-list">
                        <div>{'Action 1'}</div>

                        <div>{'Action 2'}</div>

                        <div>{'Action 3'}</div>

                        <div>{'Action 4'}</div>
                    </div>
                </nav>
            </nav>
            <div className="sidebar-footer">
                <button className="sidebar-footer-button">Apply</button>
            </div>
        </aside>
    );
};
