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

import { useGlobalLoadingState } from '../../utils/store';
import { LoadingStateSpinner } from '../../utils/LoadingStateSpinner';
import { ProductVersion } from '../../components/ProductVersion/ProductVersion';
import { TopAppBar } from '../../components/TopAppBar/TopAppBar';

export const ProductVersionPage: React.FC = () => {
    const [everythingLoaded, loadingState] = useGlobalLoadingState();
    if (!everythingLoaded) {
        return <LoadingStateSpinner loadingState={loadingState} />;
    }
    return (
        <div>
            <TopAppBar showAppFilter={true} showTeamFilter={true} showWarningFilter={false} />
            <main className="main-content">
                <ProductVersion />
            </main>
        </div>
    );
};
