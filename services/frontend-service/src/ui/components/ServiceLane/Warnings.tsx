/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/
import * as React from 'react';
import { Application, UnusualDeploymentOrder, UpstreamNotDeployed, Warning } from '../../../api/api';

export const WarningBoxes: React.FC<{ application: Application | undefined }> = (props) => {
    const { application } = props;
    if (application === undefined) {
        return <div className="warnings"></div>;
    }
    return (
        <div className="warnings">
            {application.warnings.map((warning: Warning, index: number) => (
                <div key={'warning-' + String(index)} className={'service-lane__warning'}>
                    <WarningBox warning={warning} />
                </div>
            ))}
        </div>
    );
};

export const WarningBoxUnusualDeploymentOrder: React.FC<{ warning: UnusualDeploymentOrder }> = (props) => {
    const warning = props.warning;
    const tooltip =
        warning.thisEnvironment +
        ' may be overridden with the next release train from ' +
        warning.upstreamEnvironment +
        ' to ' +
        warning.thisEnvironment +
        '. Suggestion: Create a lock on ' +
        warning.thisEnvironment +
        ' or deploy the same version to both environments.';

    return (
        <div className={'warning'} title={tooltip}>
            <b>Warning: {warning.thisEnvironment}</b> is not locked and has a newer version than{' '}
            <b>{warning.upstreamEnvironment}</b>! ⓘ
        </div>
    );
};

export const WarningUpstreamNotDeployed: React.FC<{ warning: UpstreamNotDeployed }> = (props) => {
    const warning = props.warning;
    const tooltip =
        warning.thisEnvironment +
        ' may be overridden with the next release train from ' +
        warning.upstreamEnvironment +
        ' to ' +
        warning.thisEnvironment +
        '. Suggestion: Create a lock on ' +
        warning.thisEnvironment +
        ' or deploy the same version to both environments.';

    return (
        <div className={'warning'} title={tooltip}>
            <b>Warning: {warning.upstreamEnvironment}</b> has no version deployed, but {warning.thisEnvironment} does! ⓘ
        </div>
    );
};

export const WarningBox: React.FC<{ warning: Warning }> = (props) => {
    const { warning } = props;
    switch (warning.warningType?.$case) {
        case 'unusualDeploymentOrder':
            return <WarningBoxUnusualDeploymentOrder warning={warning.warningType.unusualDeploymentOrder} />;
        case 'upstreamNotDeployed':
            return <WarningUpstreamNotDeployed warning={warning.warningType.upstreamNotDeployed} />;
        default:
            // eslint-disable-next-line no-console
            console.error('Warning type not recognized: ', JSON.stringify(warning));
            return <div>Could not render Warning</div>;
    }
};
