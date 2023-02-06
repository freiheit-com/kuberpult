import classNames from 'classnames';
import { ReactNode, useEffect, useRef } from 'react';
import { MDCList } from '@material/list';

export const NavList: React.FC<{ children?: ReactNode; className?: string }> = (props) => {
    const MDComponent = useRef<MDCList>();
    const control = useRef<HTMLElement>(null);
    const { className, children } = props;

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCList(control.current);
            MDComponent.current.wrapFocus = true;
        }
        return () => MDComponent.current?.destroy();
    }, []);

    return (
        <nav className={classNames('mdc-list', className)} ref={control}>
            {children}
        </nav>
    );
};
