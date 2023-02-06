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
