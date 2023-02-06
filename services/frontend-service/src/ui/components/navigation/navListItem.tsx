import classNames from 'classnames';
import { cloneElement, useEffect, useRef } from 'react';
import { MDCRipple } from '@material/ripple';
import { Link, useLocation } from 'react-router-dom';

export const NavbarIndicator = (props: { pathname: string; to: string }) => {
    const { pathname, to } = props;
    return (
        <div
            className={classNames('mdc-list-item-indicator', {
                'mdc-list-item-indicator--activated': pathname.startsWith(`/v2/${to}`),
            })}></div>
    );
};

export const NavListItem = (props: { className?: string; to: string; queryParams?: string; icon?: JSX.Element }) => {
    const MDComponent = useRef<MDCRipple>();
    const control = useRef<HTMLAnchorElement>(null);
    const { className, to, queryParams, icon } = props;
    const { pathname } = useLocation();

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCRipple(control.current);
        }
        return () => MDComponent.current?.destroy();
    }, []);

    const allClassNames = classNames(
        'mdc-list-item',
        { 'mdc-list-item--activated': pathname.startsWith(`/v2/${to}`) },
        className
    );

    return (
        <div className="mdc-list-item-container">
            <NavbarIndicator pathname={pathname} to={to} />
            <Link
                aria-label={to}
                className={allClassNames}
                ref={control}
                to={`${to}${queryParams ? queryParams : ''}`}
                tabIndex={pathname.startsWith(`/v2/${to}`) ? 0 : -1}>
                <div className="mdc-list-item__ripple" />
                {icon &&
                    cloneElement(icon, {
                        key: 'icon',
                    })}
            </Link>
        </div>
    );
};
