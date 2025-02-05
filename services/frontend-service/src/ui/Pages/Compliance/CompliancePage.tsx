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
import React, { useCallback } from 'react';
import { Compliance } from '../../components/Compliance/Compliance';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';

export const CompliancePage: React.FC = () => {
    const saveFile = useCallback((lines: string[]) => {
        const filename = 'deployments.csv';
        const file = new File(lines, filename);
        const anchor = document.createElement('a');
        anchor.href = URL.createObjectURL(file);
        anchor.download = filename;
        anchor.click();
        URL.revokeObjectURL(anchor.href);
    }, []);

    return (
        <div>
            <TopAppBar showAppFilter={false} showTeamFilter={false} showWarningFilter={false} />
            <Compliance saveFile={saveFile} />
        </div>
    );
};
