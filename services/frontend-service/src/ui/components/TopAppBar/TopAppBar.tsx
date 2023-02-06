import { MDCTopAppBar } from '@material/top-app-bar';

import { Textfield } from '../textfield';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { SideBar } from '../SideBar/SideBar';
import { Button } from '../button';
import { ShowBarWhite } from '../../../images';
import { useSearchParams } from 'react-router-dom';
import { Dropdown } from '../dropdown/dropdown';
import classNames from 'classnames';

export const TopAppBar: React.FC = () => {
    const control = useRef<HTMLDivElement>(null);
    const MDComponent = useRef<MDCTopAppBar>();
    const [sideBar, showSideBar] = useState(false);
    const [params] = useSearchParams();

    const toggleSideBar = useCallback(() => showSideBar((old) => !old), [showSideBar]);

    const query = params.get('application') || undefined;

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCTopAppBar(control.current);
        }
        return () => MDComponent.current?.destroy();
    }, []);

    return (
        <div className="mdc-top-app-bar" ref={control}>
            <div className="mdc-top-app-bar__row">
                <div className="mdc-top-app-bar__section mdc-top-app-bar__section--align-start">
                    <span className="mdc-top-app-bar__title">Kuberpult</span>
                </div>
                <div className="mdc-top-app-bar__section">
                    <Textfield
                        className={'top-app-bar-search-field'}
                        floatingLabel={'Application Name'}
                        value={query}
                        leadingIcon={'search'}
                    />
                    <Dropdown className={'top-app-bar-search-field'} floatingLabel={'Teams'} leadingIcon={'search'} />
                </div>
                <div className="mdc-top-app-bar__section mdc-top-app-bar__section--align-end">
                    <strong className="sub-headline1">Planned Actions</strong>
                    <Button className="mdc-show-button" icon={<ShowBarWhite />} onClick={toggleSideBar} />
                    <SideBar
                        className={classNames(`mdc-drawer-sidebar mdc-drawer-sidebar-container`, {
                            'mdc-drawer-sidebar--displayed': sideBar,
                            'mdc-drawer-sidebar--hidden': !sideBar,
                        })}
                        toggleSidebar={toggleSideBar}
                    />
                </div>
            </div>
        </div>
    );
};
