import { Logo, Home, Environments, LocksWhite } from '../../../images';
import { NavList, NavListItem } from '../navigation';
import { useLocation } from 'react-router-dom';

export const NavigationBar: React.FC = () => {
    const location = useLocation();
    return (
        <aside className="mdc-drawer">
            <div className="kp-logo">
                <Logo />
            </div>
            <div className="mdc-drawer__content">
                <NavList>
                    <NavListItem to={'home'} queryParams={location?.search || ''} icon={<Home />} />
                    <NavListItem to={'environments'} queryParams={location?.search || ''} icon={<Environments />} />
                    <NavListItem to={'locks'} queryParams={location?.search || ''} icon={<LocksWhite />} />
                </NavList>
            </div>
        </aside>
    );
};
