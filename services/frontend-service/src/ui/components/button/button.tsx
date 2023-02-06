import { useRef, useEffect, cloneElement } from 'react';
import classNames from 'classnames';
import { MDCRipple } from '@material/ripple';

export const Button = (props: {
    disabled?: boolean;
    className?: string;
    label?: string;
    icon?: JSX.Element;
    onClick?: () => void;
}) => {
    const MDComponent = useRef<MDCRipple>();
    const control = useRef<HTMLButtonElement>(null);
    const { disabled, className, label, icon, onClick } = props;

    useEffect(() => {
        if (control.current) {
            MDComponent.current = new MDCRipple(control.current);
        }
        return () => MDComponent.current?.destroy();
    }, []);

    return (
        <button
            disabled={disabled}
            className={classNames('mdc-button', className)}
            onClick={onClick}
            ref={control}
            aria-label={label || ''}>
            <div className="mdc-button__ripple" />
            {icon &&
                cloneElement(icon, {
                    key: 'icon',
                })}
            {!!label && (
                <span key="label" className="mdc-button__label">
                    {label}
                </span>
            )}
        </button>
    );
};
