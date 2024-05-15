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
import { RolloutStatus } from '../../../api/api';

export const RolloutStatusDescription: React.FC<{ status: RolloutStatus }> = (props) => {
    const { status } = props;
    switch (status) {
        case RolloutStatus.ROLLOUT_STATUS_SUCCESFUL:
            return <span className="rollout__description_successful">✓ Done</span>;
        case RolloutStatus.ROLLOUT_STATUS_PROGRESSING:
            return <span className="rollout__description_progressing">↻ In progress</span>;
        case RolloutStatus.ROLLOUT_STATUS_PENDING:
            return <span className="rollout__description_pending">⧖ Pending</span>;
        case RolloutStatus.ROLLOUT_STATUS_ERROR:
            return <span className="rollout__description_error">! Failed</span>;
        case RolloutStatus.ROLLOUT_STATUS_UNHEALTHY:
            return <span className="rollout__description_unhealthy">⚠ Unhealthy</span>;
    }
    return <span className="rollout__description_unknown">? Unknown</span>;
};
