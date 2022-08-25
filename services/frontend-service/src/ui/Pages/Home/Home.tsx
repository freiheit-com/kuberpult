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
import { Button } from '../../components/button';
import { ReleaseCard } from '../../ReleaseCard/ReleaseCard';

export const Home: React.FC = () => (
    <main className="main-content">
        <ReleaseCard />
        <ReleaseCard />
        <ReleaseCard />

        <Button label={'Button 1'} />
        <Button label={'Button 2'} />
        <Button label={'Button 3'} />
        {dummyText}
    </main>
);

const dummyText =
    'Lorem ipsum dolor sit amet, consectetur adipiscing elit. Mauris imperdiet venenatis lorem, a convallis mauris convallis nec. Duis vel laoreet tellus. Fusce egestas sem eu accumsan pellentesque. Vivamus semper nunc sit amet felis posuere, vel pharetra ante pretium. Integer aliquet nunc est, et varius est interdum non. Nulla nisi turpis, viverra sit amet ligula non, porta facilisis ante. Nunc feugiat nisi tortor, at vestibulum libero varius at. Duis laoreet orci non facilisis bibendum. Sed fermentum viverra metus vitae tincidunt. Suspendisse potenti. Aenean rhoncus sollicitudin dolor at sodales.\n' +
    '\n' +
    'Maecenas vestibulum nibh dignissim mattis vestibulum. Vivamus at consequat leo. Suspendisse cursus lacus nec diam consequat, sed pretium purus varius. Vestibulum eget tortor elementum, porta metus eu, porta arcu. Integer accumsan odio tortor, eget volutpat tellus luctus id. Nunc dolor ex, faucibus sit amet odio vel, fringilla tristique elit. In dui velit, iaculis vel tempor non, interdum et libero. Nulla et cursus velit, ut finibus eros. Maecenas tempor sapien nec nibh imperdiet, consequat porta sapien vehicula. Pellentesque iaculis nisi risus, sit amet ultricies tortor pulvinar ac. Duis vel pulvinar eros, vel luctus turpis. Fusce ante turpis, venenatis quis ipsum nec, vulputate imperdiet tellus.\n' +
    '\n' +
    'Pellentesque consequat massa nec ex elementum maximus. Nulla imperdiet imperdiet fermentum. Vestibulum tincidunt magna a magna aliquam, aliquet imperdiet justo sodales. Nullam ut ligula purus. Ut elementum, nisl sit amet posuere consectetur, tortor enim laoreet arcu, ut tincidunt est felis vel nunc. Nam nec quam enim. Integer eget quam quis nulla aliquam tempus et quis ipsum. Nunc tincidunt enim felis, a vestibulum ex pretium at.\n' +
    '\n' +
    'Nullam quis metus viverra, dictum metus a, ultrices libero. In vehicula, eros id lobortis volutpat, orci nibh rhoncus orci, vel accumsan est risus non justo. Donec convallis, tellus et consequat iaculis, ligula mauris tristique ligula, non aliquet sem sapien vitae turpis. Donec at molestie ligula. Maecenas feugiat lacinia urna, sit amet commodo eros dignissim id. Mauris eget turpis lectus. Pellentesque in tincidunt leo.\n' +
    '\n' +
    'Sed id ipsum eu enim vehicula accumsan. Ut quis erat erat. Donec neque eros, lobortis at vehicula sit amet, facilisis in libero. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Proin vitae eleifend orci. Nam porttitor eros vitae ante scelerisque, eget dignissim quam pulvinar. Quisque id iaculis odio, et molestie arcu. Vestibulum iaculis at neque ac venenatis. Cras lobortis consequat massa a facilisis. Quisque id massa sed tellus ullamcorper vulputate.\n' +
    '\n' +
    'In a luctus mi. Integer et leo viverra, ullamcorper nisi eu, mattis ipsum. Donec elit leo, congue id nibh quis, elementum pretium augue. Donec bibendum euismod convallis. Curabitur ac faucibus libero. Donec varius feugiat nisl, quis feugiat purus dignissim et. Nam varius vestibulum porttitor. Etiam sed leo eget sapien pellentesque porttitor. Maecenas non aliquet diam. Vivamus elit quam, congue vel lectus eu, ornare malesuada erat. Ut sed rutrum nunc, et viverra sem.\n' +
    '\n' +
    'Cras tristique sit amet ligula nec lobortis. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Nunc laoreet ante ut quam hendrerit condimentum. Morbi pellentesque rutrum elit, eget hendrerit mi rutrum fermentum. Mauris rhoncus tortor commodo risus pellentesque faucibus. Praesent pellentesque id est et rhoncus. Vivamus lacus tortor, euismod vel viverra a, convallis quis nunc. Suspendisse venenatis mi ex, ut vestibulum felis ultrices vel. Nam ut velit vel metus dapibus ornare in et sapien.\n' +
    '\n' +
    'Vivamus sed vulputate leo, volutpat consequat nulla. Aliquam hendrerit nec nisl vitae rutrum. Vestibulum eget risus posuere, pharetra nisl quis, tincidunt quam. Vivamus congue egestas laoreet. Cras mollis ultrices nisi sed porta. Nunc luctus lacus dui. Etiam urna tellus, tincidunt nec lorem eu, accumsan accumsan tellus. Etiam pretium risus in eros faucibus lobortis vel pulvinar metus. Phasellus et urna vulputate, luctus neque sed, elementum massa. Aliquam non massa vehicula, pretium urna sit amet, accumsan sem. Aenean in aliquam massa. Praesent imperdiet diam sed risus mattis, quis elementum ex convallis. Proin nulla tellus, molestie id elit nec, vulputate dapibus ligula.\n' +
    '\n' +
    'Morbi ac erat in ex tempus volutpat. Nunc lectus ligula, ornare vel lobortis eget, ultricies sit amet magna. Vivamus neque ligula, posuere sed maximus eu, tincidunt eu mauris. Fusce est est, convallis vel dui eget, dapibus ultricies sem. Mauris id dui felis. Nulla at ultricies urna. Sed a enim enim. Nulla eget tincidunt eros. Maecenas erat odio, fermentum sed urna sit amet, dictum pellentesque felis. Quisque ac velit ac leo convallis ullamcorper et eget purus. Nullam pellentesque risus ac dapibus congue.\n' +
    '\n' +
    'Nunc tempor nisi vitae libero maximus aliquet. Integer ut pretium felis, et malesuada neque. Quisque et risus et ante eleifend aliquam. Duis dictum lorem ut tellus blandit feugiat. Donec nisl ligula, ultricies vel pharetra sed, sollicitudin non nisl. Aliquam non varius ante. Morbi tincidunt elit non felis semper, nec varius odio ullamcorper. Suspendisse quis elit ut libero mollis euismod id iaculis nibh. Nam sit amet condimentum nulla.\n' +
    '\n' +
    'Fusce iaculis sit amet tellus eu efficitur. Aenean tempor, sem et egestas lacinia, metus ante sodales erat, et condimentum felis leo ac purus. Donec bibendum metus sem, blandit ultricies massa sagittis sed. Vestibulum pellentesque rhoncus justo. Donec non mattis ex, et pellentesque velit. Suspendisse mattis porttitor arcu. Donec sed mauris elementum, varius ante eget, luctus ipsum. Cras at lectus fringilla, malesuada justo ut, sodales arcu. Suspendisse at auctor nunc. Pellentesque gravida magna sit amet nisl ornare, at faucibus turpis sagittis. Phasellus mollis libero vitae egestas efficitur. Morbi mattis ligula ac sem bibendum, ac lacinia quam viverra. Nunc massa turpis, suscipit vel libero et, commodo blandit metus. Sed dapibus quis felis nec mollis.\n' +
    '\n' +
    'Cras maximus placerat leo in efficitur. Sed facilisis erat et mauris tincidunt, nec iaculis arcu varius. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Nunc sodales enim faucibus tellus pretium tincidunt. Vivamus dui est, interdum a tincidunt non, ullamcorper vitae ipsum. Nulla lacus odio, condimentum ac vulputate et, sodales eget urna. Morbi hendrerit nisl ut ipsum varius accumsan. Sed ac purus consequat, vehicula dui nec, tristique lorem. Phasellus mollis pulvinar fermentum. Nam posuere tempor velit eu lobortis. Mauris semper orci nulla, a fermentum eros lobortis eu. Aenean rutrum fringilla mi, quis hendrerit dolor volutpat at. Praesent id justo massa. Suspendisse eleifend rutrum bibendum. Aenean blandit ex non lorem rhoncus, non blandit risus vehicula.\n' +
    '\n' +
    'Duis dictum, justo eu pellentesque pulvinar, arcu leo aliquam libero, vel bibendum velit tellus sed mi. Duis sit amet faucibus ipsum, id tempor lacus. Donec et rutrum lacus. Donec vitae dolor vitae eros tincidunt volutpat. Nulla congue, velit nec varius malesuada, libero justo faucibus tortor, bibendum lobortis diam nibh ac lacus. Nam massa elit, consequat et pretium nec, tempus eget orci. Aenean eget ex at velit vehicula luctus a id purus. Integer sit amet purus a ligula viverra blandit a in justo. Morbi viverra eros quis velit vulputate, quis dictum augue viverra. Integer facilisis auctor imperdiet. Vestibulum quis purus vulputate, rhoncus sapien finibus, fermentum velit. Mauris dictum sapien erat, quis venenatis nunc pretium nec.\n' +
    '\n' +
    'Curabitur elementum lacinia diam, sed cursus lorem posuere at. Vestibulum aliquam nunc eu sollicitudin sodales. Duis ultrices vestibulum felis, at ultrices erat dignissim sit amet. Etiam massa dui, dictum vitae posuere quis, lacinia quis metus. Integer metus massa, vehicula quis accumsan nec, convallis molestie diam. Donec varius, urna eget egestas iaculis, diam magna interdum mauris, nec porttitor magna leo ut sapien. Nulla lobortis, ante quis lacinia tincidunt, augue ante tincidunt mauris, et accumsan ipsum tellus vitae dolor. Phasellus non pretium libero, vitae porta metus. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Vivamus eleifend libero vitae enim feugiat pharetra id ut mi. In hac habitasse platea dictumst. Nam vulputate eros eget ipsum pharetra, a dignissim justo ornare.\n' +
    '\n' +
    'Suspendisse dignissim pellentesque quam ac accumsan. Donec orci ipsum, tristique faucibus enim sit amet, placerat finibus dui. Sed placerat orci a velit molestie bibendum. Praesent laoreet tincidunt orci at ornare. Aenean sapien odio, tincidunt at erat ac, efficitur malesuada ex. Sed cursus imperdiet eleifend. Aliquam erat volutpat. Etiam in dignissim tortor. Duis lobortis, metus vel ultricies laoreet, massa ante viverra ex, eu egestas dolor orci in neque. Integer eu venenatis lorem.\n' +
    '\n' +
    'Suspendisse dictum lectus vel est tristique, at pellentesque sem fermentum. Nam eleifend ante at ante commodo blandit. Vestibulum a nulla at massa hendrerit placerat. Vivamus accumsan libero arcu, at volutpat erat sollicitudin efficitur. Sed non ligula eu tortor bibendum porttitor non sed diam. Morbi feugiat scelerisque metus. Nullam faucibus tempus nulla sit amet laoreet. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Proin tristique nisi in felis venenatis, ut suscipit diam sollicitudin. Curabitur vitae lacus nec nunc commodo malesuada vel at tortor. Nulla sed odio quis urna cursus ornare. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Ut dignissim posuere accumsan. Phasellus non mollis nisi.\n' +
    '\n' +
    'Proin turpis arcu, commodo a porttitor at, tristique vitae odio. Mauris sed scelerisque augue. Donec tempor, mauris et semper placerat, nulla odio luctus leo, vulputate pulvinar massa urna sed metus. Donec accumsan porta ligula, a aliquam libero fermentum non. Aliquam erat volutpat. Nulla sit amet sollicitudin magna, vitae viverra nunc. Nullam a aliquam mi. Fusce ac iaculis mi. Pellentesque id dolor sit amet velit pretium vehicula id molestie nibh. Suspendisse laoreet lectus faucibus, hendrerit justo in, ultricies urna. Donec non sapien viverra, placerat tellus non, cursus sem. Phasellus purus mi, dignissim vel mauris sed, rhoncus blandit lorem.\n' +
    '\n' +
    'Sed et facilisis nunc, dignissim pulvinar nibh. Proin consequat ex sed eleifend luctus. Vivamus a hendrerit sem, ut interdum nisl. Donec ex odio, fermentum eu turpis sit amet, vehicula accumsan sem. Nulla tincidunt nisi nec ex venenatis, elementum congue mi varius. Morbi sollicitudin lacus eget quam vestibulum, sit amet fringilla sem commodo. Aenean iaculis velit ac metus ultricies, ut accumsan velit vehicula. Vestibulum posuere orci ac pretium dignissim. Aliquam eu arcu sit amet neque efficitur dapibus nec ac nisi. Donec consectetur lacus non ipsum laoreet, et elementum odio congue. Vivamus sed diam ut ex luctus rutrum vitae in nulla. Nam varius, nisl quis facilisis efficitur, neque eros elementum nunc, non viverra nisl sem at lectus.\n' +
    '\n' +
    'Suspendisse congue ultrices eleifend. Mauris faucibus lectus id nibh vehicula, in convallis quam rutrum. Nulla vitae tellus massa. Phasellus vitae eros nisi. Cras vulputate purus nec tincidunt elementum. Etiam dignissim laoreet eros, quis lacinia ante volutpat eget. Nullam at ullamcorper lacus. In hac habitasse platea dictumst. Donec quis rutrum massa, sed laoreet magna. Curabitur nec libero elit. Fusce varius sapien quis arcu elementum dictum. Integer sed ullamcorper purus, eu sodales leo. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae;\n' +
    '\n' +
    'Interdum et malesuada fames ac ante ipsum primis in faucibus. Phasellus commodo vel nisi vel convallis. Maecenas vitae lorem consequat, mattis metus quis, fermentum eros. Curabitur dictum cursus ex ut gravida. Donec convallis quis neque a tristique. Suspendisse potenti. Proin vitae consequat justo. Aliquam in felis id risus vehicula gravida sed nec enim.';
