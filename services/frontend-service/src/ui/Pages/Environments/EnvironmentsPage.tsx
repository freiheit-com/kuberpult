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

Copyright 2023 freiheit.com*/
import { useEnvironmentNames } from '../../utils/store';
import { EnvironmentCard } from '../../components/EnvironmentCard/EnvironmentCard';

export const EnvironmentsPage: React.FC = () => {
    const envs = useEnvironmentNames();

    return (
        <main className="main-content">
            {envs.map((env) => (
                <EnvironmentCard environment={env} key={env} />
            ))}
        </main>
    );
};
