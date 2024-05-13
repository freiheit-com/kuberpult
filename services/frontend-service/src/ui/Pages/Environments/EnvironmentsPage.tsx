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
import { useEnvironmentGroups, useEnvironments, useGlobalLoadingState } from '../../utils/store';
import { EnvironmentCard, EnvironmentGroupCard } from '../../components/EnvironmentCard/EnvironmentCard';
import { LoadingStateSpinner } from '../../utils/LoadingStateSpinner';
import React from 'react';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';

export const EnvironmentsPage: React.FC = () => {
    const envsGroups = useEnvironmentGroups();
    const envs = useEnvironments();
    // note that in all cases, envsGroups.length <= envs.length
    // if they are equal (envsGroups.length === envs.length), then there are effectively no groups, but the cd-server still returns each env wrapped in a group
    const useGroups = envsGroups.length !== envs.length;

    const [everythingLoaded, loadingState] = useGlobalLoadingState();
    if (!everythingLoaded) {
        return <LoadingStateSpinner loadingState={loadingState} />;
    }

    const mainContent: JSX.Element = useGroups ? (
        <main className="main-content">
            {envsGroups.map((envGroup) => (
                <EnvironmentGroupCard environmentGroup={envGroup} key={envGroup.environmentGroupName} />
            ))}
        </main>
    ) : (
        <main className="main-content">
            {/*if there are no groups, wrap everything in one group: */}
            <div className="environment-group-lane">
                {envs.map((env) => (
                    <EnvironmentCard environment={env} key={env.name} group={undefined} />
                ))}
            </div>
        </main>
    );
    return (
        <div>
            <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
            {mainContent}
        </div>
    );
};
