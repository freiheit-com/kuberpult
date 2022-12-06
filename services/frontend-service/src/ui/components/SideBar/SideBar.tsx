/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
import { Button } from '../button';
import { DeleteGray, HideBarWhite } from '../../../images';
import { BatchAction } from '../../../api/api';
import { deleteAction, useActions, deleteAllActions } from '../../utils/store';
import { ChangeEvent, useCallback, useState } from 'react';
import { useApi } from '../../utils/GrpcApi';
import { TextField, Dialog, DialogTitle, DialogActions } from '@material-ui/core';

export enum ActionTypes {
    Deploy,
    PrepareUndeploy,
    Undeploy,
    CreateEnvironmentLock,
    DeleteEnvironmentLock,
    CreateApplicationLock,
    DeleteApplicationLock,
    UNKNOWN,
}

type ActionDetails = {
    type: ActionTypes;
    name: string;
    summary: string;
    dialogTitle: string;
    description?: string;

    // action details optional
    environment?: string;
    application?: string;
    lockId?: string;
    lockMessage?: string;
    version?: number;
};

const getActionDetails = ({ action }: BatchAction): ActionDetails => {
    switch (action?.$case) {
        case 'createEnvironmentLock':
            return {
                type: ActionTypes.CreateEnvironmentLock,
                name: 'Create Env Lock',
                dialogTitle: 'Are you sure you want to add this environment lock?',
                summary: 'Create new environment lock on ' + action.createEnvironmentLock.environment,
                environment: action.createEnvironmentLock.environment,
            };
        case 'deleteEnvironmentLock':
            return {
                type: ActionTypes.DeleteEnvironmentLock,
                name: 'Delete Env Lock',
                dialogTitle: 'Are you sure you want to delete this environment lock?',
                summary: 'Delete environment lock on ' + action.deleteEnvironmentLock.environment,
                environment: action.deleteEnvironmentLock.environment,
                lockId: action.deleteEnvironmentLock.lockId,
            };
        case 'createEnvironmentApplicationLock':
            return {
                type: ActionTypes.CreateApplicationLock,
                name: 'Create App Lock',
                dialogTitle: 'Are you sure you want to add this application lock?',
                summary:
                    'Lock "' +
                    action.createEnvironmentApplicationLock.application +
                    '" on ' +
                    action.createEnvironmentApplicationLock.environment,
                environment: action.createEnvironmentApplicationLock.environment,
                application: action.createEnvironmentApplicationLock.application,
            };
        case 'deleteEnvironmentApplicationLock':
            return {
                type: ActionTypes.DeleteApplicationLock,
                name: 'Delete App Lock',
                dialogTitle: 'Are you sure you want to delete this application lock?',
                summary:
                    'Unlock "' +
                    action.deleteEnvironmentApplicationLock.application +
                    '" on ' +
                    action.deleteEnvironmentApplicationLock.environment,
                environment: action.deleteEnvironmentApplicationLock.environment,
                application: action.deleteEnvironmentApplicationLock.application,
                lockId: action.deleteEnvironmentApplicationLock.lockId,
            };
        case 'deploy':
            return {
                type: ActionTypes.Deploy,
                name: 'Deploy',
                dialogTitle: 'Please be aware:',
                summary:
                    'Deploy version ' +
                    action.deploy.version +
                    ' of "' +
                    action.deploy.application +
                    '" to ' +
                    action.deploy.environment,
                environment: action.deploy.environment,
                application: action.deploy.application,
                version: action.deploy.version,
            };
        case 'prepareUndeploy':
            return {
                type: ActionTypes.PrepareUndeploy,
                name: 'Prepare Undeploy',
                dialogTitle: 'Are you sure you want to start undeploy?',
                description:
                    'The new version will go through the same cycle as any other versions' +
                    ' (e.g. development->staging->production). ' +
                    'The behavior is similar to any other version that is created normally.',
                summary: 'Prepare undeploy version for Application "' + action.prepareUndeploy.application + '"',
                application: action.prepareUndeploy.application,
            };
        case 'undeploy':
            return {
                type: ActionTypes.Undeploy,
                name: 'Undeploy',
                dialogTitle: 'Are you sure you want to undeploy this application?',
                description: 'This application will be deleted permanently',
                summary: 'Undeploy and delete Application "' + action.undeploy.application + '"',
                application: action.undeploy.application,
            };
        default:
            return {
                type: ActionTypes.UNKNOWN,
                name: 'invalid',
                dialogTitle: 'invalid',
                summary: 'invalid',
            };
    }
};

type SideBarListItemProps = {
    children: BatchAction;
};

export const SideBarListItem: React.FC<{ children: BatchAction }> = ({ children: action }: SideBarListItemProps) => {
    const actionDetails = getActionDetails(action);

    const handleDelete = useCallback(() => deleteAction(action), [action]);
    return (
        <>
            <div className="mdc-drawer-sidebar-list-item-text">
                <div className="mdc-drawer-sidebar-list-item-text-name">{actionDetails.name}</div>
                <div className="mdc-drawer-sidebar-list-item-text-summary">{actionDetails.summary}</div>
            </div>
            <div onClick={handleDelete}>
                <DeleteGray className="mdc-drawer-sidebar-list-item-delete-icon" />
            </div>
        </>
    );
};

export const SideBarList = () => {
    const actions = useActions();

    return (
        <>
            {actions.map((action, key) => (
                <div key={key} className="mdc-drawer-sidebar-list-item">
                    <SideBarListItem>{action}</SideBarListItem>
                </div>
            ))}
        </>
    );
};

export const SideBar: React.FC<{ className: string; toggleSidebar: () => void }> = (props) => {
    const { className, toggleSidebar } = props;
    const actions = useActions();
    const [lockMessage, setLockMessage] = useState('');
    const api = useApi;
    const [open, setOpen] = useState(false);

    const handleClose = useCallback(() => setOpen(false), []);
    const handleOpen = () => setOpen(true);

    const lockCreationList = actions.filter(
        (action) =>
            action.action?.$case === 'createEnvironmentLock' ||
            action.action?.$case === 'createEnvironmentApplicationLock'
    );

    const applyActions = useCallback(() => {
        if (lockMessage) {
            lockCreationList.forEach((action) => {
                if (action.action?.$case === 'createEnvironmentLock') {
                    action.action.createEnvironmentLock.message = lockMessage;
                }
                if (action.action?.$case === 'createEnvironmentApplicationLock') {
                    action.action.createEnvironmentApplicationLock.message = lockMessage;
                }
            });
            setLockMessage('');
        }
        api.batchService()
            .ProcessBatch({ actions })
            .then((result) => {
                deleteAllActions();
            });
        handleClose();
    }, [actions, api, handleClose, lockCreationList, lockMessage]);

    const newLocksExist = lockCreationList.length !== 0;

    const updateMessage = useCallback((e: ChangeEvent<HTMLInputElement>) => {
        setLockMessage(e.target.value);
    }, []);

    const canApply = actions.length > 0 && (!newLocksExist || lockMessage);

    return (
        <aside className={className}>
            <nav className="mdc-drawer-sidebar mdc-drawer__drawer sidebar-content">
                <div className="mdc-drawer-sidebar mdc-drawer-sidebar-header">
                    <Button
                        className={'mdc-drawer-sidebar mdc-drawer-sidebar-header mdc-drawer-sidebar-header__button'}
                        icon={<HideBarWhite />}
                        onClick={toggleSidebar}
                    />
                    <h1 className="mdc-drawer-sidebar mdc-drawer-sidebar-header-title">Planned Actions</h1>
                </div>
                <nav className="mdc-drawer-sidebar mdc-drawer-sidebar-content">
                    <div className="mdc-drawer-sidebar mdc-drawer-sidebar-list">
                        <SideBarList />
                    </div>
                </nav>
                {newLocksExist && (
                    <TextField
                        label="Lock Message"
                        variant="outlined"
                        placeholder="default-lock"
                        onChange={updateMessage}
                        className="actions-cart__lock-message"
                        value={lockMessage}
                    />
                )}
                <div className="mdc-drawer-sidebar mdc-sidebar-sidebar-footer">
                    <Button
                        className={
                            'mdc-drawer-sidebar mdc-sidebar-sidebar-footer mdc-drawer-sidebar-apply-button' +
                            (!canApply ? '-disabled' : '')
                        }
                        label={'Apply'}
                        onClick={canApply ? handleOpen : undefined}
                    />
                    <Dialog open={open} onClose={handleClose}>
                        <DialogTitle id="alert-dialog-title">
                            {'Are you sure you want to apply all planned actions?'}
                        </DialogTitle>
                        <DialogActions>
                            <Button label="Cancel" onClick={handleClose} />
                            <Button label="Confirm" onClick={applyActions} />
                        </DialogActions>
                    </Dialog>
                </div>
            </nav>
        </aside>
    );
};
